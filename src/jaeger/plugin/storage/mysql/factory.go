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
	"flag"
	"database/sql"

	"github.com/spf13/viper"
	"github.com/uber/jaeger-lib/metrics"
	"go.uber.org/zap"
	_ "github.com/go-sql-driver/mysql"

	"github.com/jaegertracing/jaeger/storage/spanstore"
	"github.com/jaegertracing/jaeger/storage/dependencystore"
	mSpanStore "github.com/jaegertracing/jaeger/plugin/storage/mysql/spanstore"
	depStore "github.com/jaegertracing/jaeger/plugin/storage/mysql/dependencystore"
	"github.com/jaegertracing/jaeger/plugin/storage/mysql/spanstore/dbmodel"
)

const (
	SpanDropCountName 				= "mysql_span_drop_count"
	MysqlBatchInsertErrorName       = "mysql_batch_insert_error_count"
)

// Factory implements storage.Factory and creates storage components backed by mysql store.
type Factory struct {
	options        Options
	metricsFactory metrics.Factory
	logger         *zap.Logger
	store          *sql.DB
	cacheStore     *mSpanStore.CacheStore
	backgroudStore *mSpanStore.BackgroudStore
	eventQueue     chan *dbmodel.Span

	metrics struct {
		// SpanDropCount returns the count of dropped span when the queue is full
		SpanDropCount 			metrics.Counter
		MysqlBatchInsertError   metrics.Counter
	}
}

// NewFactory creates a new Factory.
func NewFactory() *Factory {
	return &Factory{}
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

	logger.Info("Mysql storage initialized successed")
	return nil
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