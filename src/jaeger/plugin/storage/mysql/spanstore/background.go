// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2017 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spanstore

import (
	"fmt"
	"time"
	"database/sql"
	_ "strings"
	"github.com/smartwalle/dbs"

	"go.uber.org/zap"
	"github.com/uber/jaeger-lib/metrics"

	"github.com/jaegertracing/jaeger/plugin/storage/mysql/spanstore/dbmodel"
)

type BackgroudStore struct{
	mysql_client   			*sql.DB 
	eventQueue     			chan *dbmodel.Span
	logger         			*zap.Logger
	lingerTime     			time.Duration
	batchSize      			int
	workers        			int
	MysqlBatchInsertError   metrics.Counter
}

func NewBackgroudStore(client *sql.DB, ch chan *dbmodel.Span, logger *zap.Logger, lingerTime int, batch int, workers int, MysqlBatchInsertError metrics.Counter)*BackgroudStore{
	return &BackgroudStore{
		mysql_client: client,
		eventQueue: ch, 
		logger: logger,
		lingerTime: time.Duration(uint64(lingerTime)) * time.Millisecond,
		batchSize: batch,
		workers: workers,
		MysqlBatchInsertError: MysqlBatchInsertError,
	}
}

// Close closes SpanWriter
func (b BackgroudStore) Close() error {
	b.mysql_client.Close()
	return nil
}

func (b BackgroudStore)Start(){
	var (
		eventQueue     = b.eventQueue
		batchSize      = b.batchSize //default 50 
		workers        = b.workers //default 8
		lingerTime     = b.lingerTime // default 200 * time.Millisecond 
		batchProcessor = func(batch []*dbmodel.Span) error {
			if len(batch) > 0{
				b.logger.Info("process items", zap.Int("batch", len(batch)))
				err := b.batch_insert(batch)
				return err
			}else{
				b.logger.Info("batch is 0")
			}
			return nil
		}
		errHandler = func(err error, batch []*dbmodel.Span) {
			// TODO add metrics and alert
			b.logger.Error("some error happens when batch insert", zap.Error(err))  
			b.MysqlBatchInsertError.Inc(1)
		}
	)

	for i := 0; i < workers; i++ {
		go func() {
			var batch []*dbmodel.Span
			lingerTimer := time.NewTimer(0)
			if !lingerTimer.Stop() {
				<-lingerTimer.C
			}
			defer lingerTimer.Stop()

			for {
				select {
				case msg := <-eventQueue:
					batch = append(batch, msg)
					if len(batch) != batchSize {
						if len(batch) == 1 {
							lingerTimer.Reset(lingerTime)
						}
						break
					}

					b.logger.Info("batch is reach")
					if err := batchProcessor(batch); err != nil {
						errHandler(err, batch)
					}

					if !lingerTimer.Stop() {
						<-lingerTimer.C
					}

					batch = make([]*dbmodel.Span, 0)
				case <-lingerTimer.C:
					b.logger.Info("time is reach")
					if err := batchProcessor(batch); err != nil {
						errHandler(err, batch)
					}

					batch = make([]*dbmodel.Span, 0)
				}
			}
		}()
		b.logger.Info("start storage worker success", zap.Int("worker",i))
	}
}

func (b BackgroudStore)batch_insert(spans []*dbmodel.Span) error{
	var ib = dbs.NewInsertBuilder()
    ib.Table("traces")
    ib.Columns("trace_id", "span_id", "span_hash", "parent_id", "operation_name", "flags",
		"start_time", "duration", "tags", "logs", "refs", "process", "service_name", "http_code", "error")
	for _, span := range spans {
		ib.Values(span.TraceID, span.SpanID,span.SpanHash, span.ParentID, span.OperationName, span.Flags, span.StartTime,
			span.Duration, span.Tags, span.Logs, span.Refs, span.Process, span.ServiceName, span.HttpCode, span.Error)
	}
	_, err := ib.Exec(b.mysql_client)
	if err != nil {
		fmt.Println(ib.ToSQL)
		b.logger.Error("batch insert error", zap.Error(err))			
	}	
	return err	
}
