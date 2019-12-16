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
	"context"
	"database/sql"
	"fmt"
	"time"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
	"github.com/jaegertracing/jaeger/plugin/storage/mysql/spanstore/dbmodel"
)

// Store is an in-memory store of traces
type SpanReader struct {
	mysql_client  *sql.DB
	cache         *CacheStore
	logger        *zap.Logger
}

func NewSpanReader(store *sql.DB, cacheStore *CacheStore, logger *zap.Logger) *SpanReader{
	return &SpanReader{
		mysql_client: store,
		cache: cacheStore, 
		logger: logger,
	}
}

// Close closes SpanWriter
func (r *SpanReader) Close() error {
	r.mysql_client.Close()
	r.cache.Close()
	return nil
}

// GetTrace gets a trace
func (r *SpanReader) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error){
	trace := model.Trace{}
	trace_id := traceID.String()
	rows, err := r.mysql_client.Query(queryTraceByTraceId, trace_id)
	if err != nil {
		r.logger.Error("queryTrace err", zap.Error(err))
		return nil, err
	}
	defer rows.Close()
	var spans []*model.Span
	for rows.Next() {
		dbspan := new(dbmodel.Span)
		err := rows.Scan(&dbspan.TraceID, 
						 &dbspan.SpanID, 
						 &dbspan.ParentID, 
						 &dbspan.OperationName, 
						 &dbspan.Flags, 
						 &dbspan.StartTime, 
						 &dbspan.Duration, 
						 &dbspan.Tags, 
						 &dbspan.Logs, 
						 &dbspan.Refs, 
						 &dbspan.Process)
		if err != nil {
			r.logger.Error("queryTrace scan err", zap.Error(err))
		}
		span, err := dbmodel.ToDomain(dbspan)
		if err != nil {
			r.logger.Error("queryTrace scan err", zap.Error(err))
		}else{
			spans = append(spans, span)
		}
	}
	trace.Spans = spans
	return &trace, nil
}

// GetServices returns a list of all known services
func (r *SpanReader) GetServices(ctx context.Context) ([]string, error){
	return r.cache.LoadServices()
}

// GetOperations returns the operations of a given service
func (r *SpanReader) GetOperations(ctx context.Context, service string) ([]string, error){
	return r.cache.LoadOperations(service)
}

// FindTraces returns all traces in the query parameters are satisfied by a trace's span
func (r *SpanReader) FindTraces(ctx context.Context, query *spanstore.TraceQueryParameters) ([]*model.Trace, error){
	traceIds,err := r.FindTraceIDs(ctx, query) // must need FindTraceIDs because of the limit params
	if err != nil {
		r.logger.Error("FindTraceIDs err", zap.Error(err))
		return nil, err
	}
	if len(traceIds) <= 0 {
		r.logger.Info("there is no trace match the condition")
		return nil, nil
	}

	var traceIdsStr string = ""
	for _,trace_id := range traceIds {
		if traceIdsStr != "" {
			traceIdsStr = traceIdsStr + ","
		}
		traceIdsStr = traceIdsStr + "'" + trace_id.String() + "'"
	}
	traces_map := make(map[string][]*model.Span)
	SQL := queryTraceByTraceIds + "(" + traceIdsStr + ")"
	//r.logger.Info("FindTraces query sql", zap.String("SQL", SQL))

	rows, err := r.mysql_client.Query(SQL)
	defer rows.Close()
	if err != nil {
		r.logger.Error("FindTraces err", zap.Error(err))
		return nil, err
	}
	for rows.Next() {
		dbspan := new(dbmodel.Span)
		err := rows.Scan(&dbspan.TraceID, 
						 &dbspan.SpanID, 
						 &dbspan.ParentID, 
						 &dbspan.OperationName, 
						 &dbspan.Flags, 
						 &dbspan.StartTime, 
						 &dbspan.Duration, 
						 &dbspan.Tags, 
						 &dbspan.Logs, 
						 &dbspan.Refs, 
						 &dbspan.Process)
		if err != nil {
			r.logger.Error("queryTrace scan err", zap.Error(err))
		}
		spans, ok := traces_map[dbspan.TraceID]
		if !ok {
			spans = []*model.Span{}
		}
		span, err := dbmodel.ToDomain(dbspan)
		if err != nil {
			r.logger.Error("queryTrace scan err", zap.Error(err))
		}else{
			spans = append(spans, span)
			traces_map[dbspan.TraceID] = spans
		}
	}
	//r.logger.Info("traces info", zap.Any("traces_map", traces_map))
	var traces []*model.Trace 
	for _, spans := range traces_map {
		trace := model.Trace{}
		trace.Spans = spans
		traces = append(traces, &trace)
	}
	return traces, nil
}

// FindTraceIDs 
func (r *SpanReader) FindTraceIDs(ctx context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error){
	defaultQuery := gen_query_sql(query)
	r.logger.Info("defauleQuerySql", zap.String("SQL", defaultQuery))
	rows, err := r.mysql_client.Query(defaultQuery)
	defer rows.Close()
	if err != nil {
		r.logger.Error("queryTraceIDs err", zap.Error(err))
		return nil, err
	}
	var traceIds []model.TraceID
	var traceIdStr string
	for rows.Next() {
		err := rows.Scan(&traceIdStr)
		if err != nil {
			r.logger.Error("queryTraceIDs scan err", zap.Error(err))
		}
		traceId, err := model.TraceIDFromString(traceIdStr)
		if err != nil {
			r.logger.Error("queryTraceIDs TraceIDFromString err", zap.Error(err))
		}else {
			traceIds = append(traceIds, traceId)
		}
	}
	return traceIds, nil
}

func gen_query_sql(query *spanstore.TraceQueryParameters) string {
	// TODO need more graceful
	defaultQuery := "SELECT trace_id FROM traces"
	// add condition
	var conditions []string
	if query.ServiceName != ""{
		conditions = append(conditions, fmt.Sprintf("service_name='%s'", query.ServiceName))
	}
	if query.OperationName != ""{
		conditions = append(conditions, fmt.Sprintf("operation_name='%s'", query.OperationName))
	}
	var t time.Time
	if query.StartTimeMax != t {
		start_time_max := int64(model.TimeAsEpochMicroseconds(query.StartTimeMax))
		conditions = append(conditions, fmt.Sprintf("start_time<=%d", start_time_max))
	}
	if query.StartTimeMin != t {
		start_time_min := int64(model.TimeAsEpochMicroseconds(query.StartTimeMin))
		conditions = append(conditions, fmt.Sprintf("start_time>=%d", start_time_min))
	}
	if query.DurationMax > 0 {
		duration_max := int64(model.DurationAsMicroseconds(query.DurationMax))
		conditions = append(conditions, fmt.Sprintf("duration<=%d", duration_max))
	}
	if query.DurationMin > 0 {
		duration_min := int64(model.DurationAsMicroseconds(query.DurationMin))
		conditions = append(conditions, fmt.Sprintf("duration>=%d", duration_min))
	}
	if http_code, ok := query.Tags["http.status_code"]; ok {
		conditions = append(conditions, fmt.Sprintf("http_code=%s", http_code))
	}
	if isError, ok := query.Tags["error"]; ok {
		conditions = append(conditions, fmt.Sprintf("error=%v", isError))
	}

	if len(conditions) > 0{
		defaultQuery = defaultQuery + " where " + strings.Join(conditions, " AND ")
	}

	// add order
	defaultQuery = defaultQuery + " order by start_time desc"
	// default 20 if no query params
	limit := query.NumTraces
	if limit <= 0 {
		limit = 20
	}
	defaultQuery = fmt.Sprintf("SELECT trace_id FROM (%s) as tmp", defaultQuery) + fmt.Sprintf(" limit %d", limit)
	return defaultQuery
}