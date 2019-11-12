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
	"fmt"
	"encoding/json"

	"github.com/jaegertracing/jaeger/model"
)

var (
	dbToDomainRefMap = map[string]model.SpanRefType{
		childOf:     model.SpanRefType_CHILD_OF,
		followsFrom: model.SpanRefType_FOLLOWS_FROM,
	}

	domainToDBRefMap = map[model.SpanRefType]string{
		model.SpanRefType_CHILD_OF:     childOf,
		model.SpanRefType_FOLLOWS_FROM: followsFrom,
	}

	domainToDBValueTypeMap = map[model.ValueType]string{
		model.StringType:  stringType,
		model.BoolType:    boolType,
		model.Int64Type:   int64Type,
		model.Float64Type: float64Type,
		model.BinaryType:  binaryType,
	}
)

// FromDomain converts a domain model.Span to a database Span
func FromDomain(span *model.Span) *Span {
	return converter{}.fromDomain(span)
}

// ToDomain converts a database Span to a domain model.Span
func ToDomain(dbSpan *Span) (*model.Span, error) {
	return converter{}.toDomain(dbSpan)
}

// converter converts Spans between domain and database representations.
// It primarily exists to namespace the conversion functions.
type converter struct{}

func (c converter) toDomain(dbSpan *Span) (*model.Span, error) {
	traceID, err := model.TraceIDFromString(dbSpan.TraceID)
	if err != nil {
		return nil, err
	}
	refs, err := c.fromDBRefs(dbSpan.Refs, traceID)
	if err != nil {
		return nil, err
	}
	tags, err := c.fromDBTags(dbSpan.Tags)
	if err != nil {
		return nil, err
	}
	logs, err := c.fromDBLogs(dbSpan.Logs)
	if err != nil {
		return nil, err
	}
	process, err := c.fromDBProcess(dbSpan.Process)
	if err != nil {
		return nil, err
	}

	span := &model.Span{
		TraceID:       traceID,
		SpanID:        model.NewSpanID(uint64(dbSpan.SpanID)),
		OperationName: dbSpan.OperationName,
		References:    model.MaybeAddParentSpanID(traceID, model.NewSpanID(uint64(dbSpan.ParentID)), refs),
		Flags:         model.Flags(uint32(dbSpan.Flags)),
		StartTime:     model.EpochMicrosecondsAsTime(uint64(dbSpan.StartTime)),
		Duration:      model.MicrosecondsAsDuration(uint64(dbSpan.Duration)),
		Tags:          tags,
		Logs:          logs,
		Process:       process,
	}
	return span, nil
}

func (c converter) fromDBRefs(dbrefs string, traceID model.TraceID) ([]model.SpanRef, error) {
	var refs []SpanRef
	err := json.Unmarshal([]byte(dbrefs), &refs)
	if err != nil {
		return nil, err
	}
	retMe := make([]model.SpanRef, len(refs))
	for i, r := range refs {
		retMe[i] = model.SpanRef{
			TraceID: traceID,
			SpanID:  model.NewSpanID(uint64(r.SpanID)),
			RefType: dbToDomainRefMap[r.RefType],
		}
	}
	return retMe, nil
}

func (c converter) fromDBTags(tags string) ([]model.KeyValue, error) {
	retMe := []model.KeyValue{}
	err := json.Unmarshal([]byte(tags), &retMe)
	if err != nil {
		return nil, err
	}
	return retMe, nil
}

func (c converter) fromDBLogs(logs string) ([]model.Log, error) {
	retMe := []model.Log{}
	err := json.Unmarshal([]byte(logs), &retMe)
	if err != nil {
		return nil, err
	}
	return retMe, nil
}

func (c converter) fromDBProcess(process string) (*model.Process, error) {
	retMe := model.Process{}
	err := json.Unmarshal([]byte(process), &retMe)
	if err != nil {
		return nil, err
	}
	return &retMe, nil
}

func (c converter) fromDomain(span *model.Span) *Span {
	tags := c.toDBTags(span.Tags)
	logs := c.toDBLogs(span.Logs)
	refs, parent_id := c.toDBRefs(span.References)
	udtProcess := c.toDBProcess(span.Process)
	spanHash, _ := model.HashCode(span)
	http_code_tag := getHttpCode(span.Tags)
	error_tag := getError(span.Tags)

	return &Span{
		TraceID:       span.TraceID.String(),
		SpanID:        int64(span.SpanID),
		SpanHash:      int64(spanHash),
		ParentID:      parent_id,
		OperationName: span.OperationName,
		Flags:         int32(span.Flags),
		StartTime:     int64(model.TimeAsEpochMicroseconds(span.StartTime)),
		Duration:      int64(model.DurationAsMicroseconds(span.Duration)),
		Tags:          tags,
		Logs:          logs,
		Refs:          refs,
		Process:       udtProcess,
		ServiceName:   span.Process.ServiceName,
		HttpCode:      http_code_tag,
		Error:         error_tag,
	}
}

func getHttpCode(tags []model.KeyValue) int64{
	for _, tag := range tags {
		if tag.GetKey() == "http.status_code" {
			return tag.GetVInt64()
		}
	}
	return 0
}

func getError(tags []model.KeyValue) bool{
	for _, tag := range tags {
		if tag.GetKey() == "error" {
			return true
		}
	}
	return false
}

func jsonMarshal(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Println(err)
		return fmt.Sprintf("jsonMarshal Marshal error: %s", err)
	}
	return string(data)
}

func (c converter) toDBTags(tags []model.KeyValue) string {
	return jsonMarshal(tags)
}

func (c converter) toDBLogs(logs []model.Log) string {
	return jsonMarshal(logs)
}

func (c converter) toDBRefs(refs []model.SpanRef) (string, int64) {
	retMe := make([]SpanRef, len(refs))
	var parent_id int64
	for i, r := range refs {
		retMe[i] = SpanRef{
			TraceID: r.TraceID.String(),
			SpanID:  int64(r.SpanID),
			RefType: domainToDBRefMap[r.RefType],
		}
		if v := domainToDBRefMap[r.RefType]; v == childOf{
			parent_id = int64(r.SpanID)
		}
	}
	return jsonMarshal(retMe), parent_id
}

func (c converter) toDBProcess(process *model.Process) string {
	return jsonMarshal(process)
}
