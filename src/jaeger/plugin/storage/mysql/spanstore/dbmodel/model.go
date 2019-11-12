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

package dbmodel

import (
	_ "bytes"
	_"encoding/binary"
	_ "encoding/json"

	_ "github.com/jaegertracing/jaeger/model"
)

const (
	childOf     = "child-of"
	followsFrom = "follows-from"

	stringType  = "string"
	boolType    = "bool"
	int64Type   = "int64"
	float64Type = "float64"
	binaryType  = "binary"
)

// TraceID is a serializable form of model.TraceID
// type TraceID [16]byte

// Span is the database representation of a span.
type Span struct {
	TraceID       string  `db:"trace_id"`
	SpanID        int64   `db:"span_id"`
	SpanHash      int64   `db:"span_hash"`
	ParentID      int64   `db:"parent_id"`
	OperationName string  `db:"operation_name"`
	Flags         int32   `db:"flags"`
	StartTime     int64   `db:"start_time"`
	Duration      int64   `db:"duration"`
	Tags          string  `db:"tags"`
	Logs          string  `db:"logs"`
	Refs          string  `db:"refs"`
	Process       string  `db:"process"`
	ServiceName   string  `db:"service_name"`
	HttpCode      int64   `db:"http_code"`
	Error         bool    `db:"error"`
}

// SpanRef is the UDT representation of a Jaeger Span Reference.
type SpanRef struct {
	RefType string  		`json:"ref_type"`
	TraceID string   		`json:"trace_id"`
	SpanID  int64   		`json:"sapn_id"`
}
