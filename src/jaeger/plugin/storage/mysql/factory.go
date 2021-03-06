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

package mysql

import (
	"fmt"
	"database/sql"
	"flag"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"github.com/uber/jaeger-lib/metrics"
	"go.uber.org/zap"

	depStore "github.com/jaegertracing/jaeger/plugin/storage/mysql/dependencystore"
	mSpanStore "github.com/jaegertracing/jaeger/plugin/storage/mysql/spanstore"
	"github.com/jaegertracing/jaeger/plugin/storage/mysql/spanstore/dbmodel"
	"github.com/jaegertracing/jaeger/storage/dependencystore"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

const (
	SpanDropCountName         = "mysql_span_drop_count"
	MysqlBatchInsertErrorName = "mysql_batch_insert_error_count"
)

// Factory implements storage.Factory and creates storage components backed by mysql store.
type Factory struct {
	options         Options
	metricsFactory  metrics.Factory
	logger          *zap.Logger
	store           *sql.DB
	cacheStore      *mSpanStore.CacheStore
	backgroudStore  *mSpanStore.BackgroudStore
	eventQueue      chan *dbmodel.Span
	maintenanceDone chan bool

	metrics struct {
		// SpanDropCount returns the count of dropped span when the queue is full
		SpanDropCount         metrics.Counter
		MysqlBatchInsertError metrics.Counter
	}
}

// NewFactory creates a new Factory.
func NewFactory() *Factory {
	return &Factory{
		maintenanceDone: make(chan bool),
	}
}

// AddFlags implements plugin.Configurable
func (f *Factory) AddFlags(flagSet *flag.FlagSet) {
	f.options.AddFlags(flagSet)
}

// InitFromViper implements plugin.Configurable
func (f *Factory) InitFromViper(v *viper.Viper) {
	f.options.InitFromViper(v)
}

// Initialize implements storage.Factory
func (f *Factory) Initialize(metricsFactory metrics.Factory, logger *zap.Logger) error {
	f.metricsFactory, f.logger = metricsFactory, logger

	f.metrics.SpanDropCount = metricsFactory.Counter(metrics.Options{Name: SpanDropCountName})
	f.metrics.MysqlBatchInsertError = metricsFactory.Counter(metrics.Options{Name: MysqlBatchInsertErrorName})

	db, err := sql.Open("mysql", f.options.Configuration.Url) // 建立一个mysql连接对象
	if err != nil {
		logger.Fatal("Cannot create mysql session", zap.Error(err))
		return err
	}
	f.store = db

	f.cacheStore = mSpanStore.NewCacheStore(f.store, f.logger)
	f.cacheStore.Initialize()

	f.eventQueue = make(chan *dbmodel.Span, f.options.Configuration.QueueLength)
	f.backgroudStore = mSpanStore.NewBackgroudStore(f.store, f.eventQueue, f.logger, f.options.Configuration.LingerTime,
		f.options.Configuration.Batchsize, f.options.Configuration.Workers, f.metrics.MysqlBatchInsertError)
	f.backgroudStore.Start()

	go f.maintenance()

	logger.Info("Mysql storage initialized successed")
	return nil
}

// Maintenance starts a background maintenance job for the clean mysql expired data
func (f *Factory) maintenance() {
	expired := int64(f.options.Configuration.Expired * 3600 * 24)
	interval := time.Duration(f.options.Configuration.Interval) * time.Minute
	maintenanceTicker := time.NewTicker(interval)
	defer maintenanceTicker.Stop()
	for {
		select {
		case <-f.maintenanceDone:
			return
		case <-maintenanceTicker.C:
            // delete expired mysql data
			startTime := (time.Now().Unix() - expired) * 1000000
			startTime1 := startTime - 30 * 60 * 1000000   // interval time 
			sql := fmt.Sprintf("delete from traces where start_time <= %d and start_time > %d limit 1000", startTime, startTime1)
			var rowsaffectedTotal int
			for {
				rowsaffected, err := deleteMysqlExpiredData(f.store, sql)
				rowsaffectedTotal = rowsaffectedTotal + rowsaffected
				if err == nil && rowsaffected < 1000 {
					break
				}
				if err != nil {
					f.logger.Error("delete expired mysql data error", zap.Error(err))	
				}
				time.Sleep(1 * time.Second)
				f.logger.Info("delete expired mysql data", zap.Int("rowsAffected", rowsaffected))
			}

			f.logger.Info("delete expired mysql data success", zap.Int("expired(d)", f.options.Configuration.Expired), 
																zap.Int("interval(m)", f.options.Configuration.Interval),
																zap.Int("rowsaffectedTotal", rowsaffectedTotal))

			// todo metrics
		}
	}
}


func deleteMysqlExpiredData(db *sql.DB, sql string) (int, error) {
	results, err := db.Exec(sql)
	if err != nil {
		return 0, err	
	}
	rowsaffected, err := results.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rowsaffected, nil
}

// Close Implements io.Closer and closes the underlying storage
func (f *Factory) Close() error {
	close(f.maintenanceDone)
	err := f.store.Close()
	return err
}

// CreateSpanReader implements storage.Factory
func (f *Factory) CreateSpanReader() (spanstore.Reader, error) {
	return mSpanStore.NewSpanReader(f.store, f.cacheStore, f.logger), nil
}

// CreateSpanWriter implements storage.Factory
func (f *Factory) CreateSpanWriter() (spanstore.Writer, error) {
	return mSpanStore.NewSpanWriter(f.eventQueue, f.cacheStore, f.logger, f.metrics.SpanDropCount), nil
}

// CreateDependencyReader implements storage.Factory
func (f *Factory) CreateDependencyReader() (dependencystore.Reader, error) {
	sr, _ := f.CreateSpanReader() // err is always nil
	return depStore.NewDependencyStore(sr), nil
}
