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
	"go.uber.org/zap"
	"github.com/uber/jaeger-lib/metrics"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/plugin/storage/mysql/spanstore/dbmodel"
)

// SpanWriter 
type SpanWriter struct {
	eventQueue    chan *dbmodel.Span
	cache         *CacheStore
	logger        *zap.Logger
	WriteMetrics  
}

type WriteMetrics struct {
	dropSpanCount      metrics.Counter
}

func NewWriteMetrics(dropSpanCounter metrics.Counter) WriteMetrics{
	return WriteMetrics{
		dropSpanCount: dropSpanCounter,
	}
}

func NewSpanWriter(ch chan *dbmodel.Span, cacheStore *CacheStore, logger *zap.Logger, dropSpanCounter metrics.Counter) *SpanWriter{
	writeMetrics := NewWriteMetrics(dropSpanCounter)
	return &SpanWriter{
		eventQueue: ch,
		cache: cacheStore,
		logger: logger,
		WriteMetrics: writeMetrics,
	}
}

// Close closes SpanWriter
func (w *SpanWriter) Close() error {
	close(w.eventQueue)
	w.cache.Close()
	return nil
}

// WriteSpan writes the given span
func (w *SpanWriter) WriteSpan(span *model.Span) error {
	ds := dbmodel.FromDomain(span)
	select {
	case w.eventQueue <- ds:
		w.logger.Info("sent one span")
	default:
		// report metric
		w.logger.Error("no span sent")
		w.dropSpanCount.Inc(1)
	}

	// use cache to save the less data, note to load the data to cache when start init 
	w.cache.UpdateCaches(ds.ServiceName, ds.OperationName)

	return nil
}
