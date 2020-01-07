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
	"database/sql"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)


const (
	insertSpan = `INSERT INTO traces(trace_id, span_id, span_hash, parent_id, operation_name, flags,
				    start_time, duration, tags, logs, refs, process, service_name)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	insertServiceName = `INSERT ignore INTO service_names(service_name) VALUES (?)`
	insertOperationName = `INSERT ignore  INTO operation_names(service_name, operation_name) VALUES (?, ?)`
	queryTraceByTraceId = `SELECT trace_id,span_id,parent_id,operation_name,flags,start_time,duration,tags,logs,refs,process FROM traces where trace_id = ?`
	queryTraceByTraceIds = "SELECT trace_id,span_id,parent_id,operation_name,flags,start_time,duration,tags,logs,refs,process FROM traces where trace_id in "
	queryServiceNames = `SELECT service_name FROM service_names`
	queryOperationsByServiceName = `SELECT operation_name FROM operation_names where service_name = ?`
)

// CacheStore 
type CacheStore struct {
	mysql_client  *sql.DB
	caches        map[string]map[string]struct{}
	cacheLock     sync.Mutex
	logger        *zap.Logger
}

// Close closes SpanWriter
func (c *CacheStore) Close() error {
	c.mysql_client.Close()
	return nil
}

// NewCacheStore 
func NewCacheStore(mysql_client *sql.DB, logger *zap.Logger) *CacheStore {
	return &CacheStore{
		mysql_client: mysql_client, 
		caches: map[string]map[string]struct{}{},
		logger: logger,
	}
}

func (c *CacheStore)Initialize(){
	c.load_caches()
}

func (c *CacheStore)load_caches(){
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	service_names, err := c.LoadServices()
	if err != nil {
		c.logger.Error("getServices error", zap.Error(err))
		return 
	}
	for _, service_name := range service_names {
		c.caches[service_name] = map[string]struct{}{}
		operation_names, err := c.LoadOperations(service_name)
		if err != nil {
			c.logger.Error("get service operation error", zap.Error(err))
			continue 
		}
		for _, operation_name := range operation_names{
			c.caches[service_name][operation_name] = struct{}{}
		}
	}
	c.logger.Info("load caches success", zap.Any("caches", c.caches))
}

func (c *CacheStore) UpdateCaches(service string, operation string){
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	service_operations, ok := c.caches[service]
	if !ok {
		c.caches[service] = map[string]struct{}{}
		c.caches[service][operation] = struct{}{}
		// insert service operation to mysql
		_, err := c.mysql_client.Exec(insertServiceName, service)
		if err != nil {
			c.logger.Error("write service_name err", zap.Error(err))
		}
	}else{
		if _, ok := service_operations[operation]; !ok{
			c.caches[service][operation] = struct{}{}
			// insert operation to mysql
			_, err := c.mysql_client.Exec(insertOperationName, service, operation)
			if err != nil {
				c.logger.Error("write operation_name err", zap.Error(err))
			}
		}
	}
}

func (c *CacheStore)LoadServices()([]string, error){
	rows, err := c.mysql_client.Query(queryServiceNames)
	if err != nil {
		c.logger.Error("queryService err", zap.Error(err))
		return nil, err
	}
	defer rows.Close()
	var service_names []string
	var service_name string
	for rows.Next() {
		err := rows.Scan(&service_name)
		if err != nil {
			c.logger.Error("queryService scan err", zap.Error(err))
		}
		service_names = append(service_names, service_name)
	}
	return service_names, nil
}

func (c *CacheStore) LoadOperations(service string) ([]string, error){
	rows, err := c.mysql_client.Query(queryOperationsByServiceName, service)
	if err != nil {
		c.logger.Error("queryOperation err", zap.Error(err))
		return nil, err
	}
	defer rows.Close()
	var operation_names []string
	var operation_name string
	for rows.Next() {
		err := rows.Scan(&operation_name)
		if err != nil {
			c.logger.Error("queryService scan err", zap.Error(err))
		}
		operation_names = append(operation_names, operation_name)
	}
	return operation_names, nil
}
