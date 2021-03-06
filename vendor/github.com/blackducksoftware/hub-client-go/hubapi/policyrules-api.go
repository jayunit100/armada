// Copyright 2018 Synopsys, Inc.
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

package hubapi

import (
	"encoding/json"
	"fmt"
)

type PolicyRuleList struct {
	TotalCount uint32       `json:"totalCount"`
	Items      []PolicyRule `json:"items"`
	Meta       Meta         `json:"_meta"`
}

type PolicyRule struct {
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	Enabled       bool             `json:"enabled"`
	Overridable   bool             `json:"overridable"`
	Severity      string           `json:"severity"`
	Expression    PolicyExpression `json:"expression"`
	CreatedAt     string           `json:"createdAt"`
	CreatedBy     string           `json:"createdBy"`
	CreatedByUser string           `json:"createdByUser"`
	UpdatedAt     string           `json:"updatedAt"`
	UpdatedBy     string           `json:"updatedBy"`
	UpdatedByUser string           `json:"updatedByUser"`
	Meta          Meta             `json:"_meta"`
}

type PolicyExpression struct {
	Operator    string       `json:"operator"`
	Expressions []Expression `json:"expressions"`
}

type Expression struct {
	Name       string              `json:"name"`
	Operation  string              `json:"operation"`
	Parameters ExpressionParameter `json:"parameters"`
}

type ExpressionParameter struct {
	Values []string                 `json:"values"`
	Data   []map[string]IntOrString `json:"data"`
}

type PolicyRuleRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Enabled     bool             `json:"enabled"`
	Overridable bool             `json:"overridable"`
	Expression  PolicyExpression `json:"expression"`
	Severity    string           `json:"severity"`
}

type IntOrString struct {
	Type   Type   `json:"type"`
	IntVal int32  `json:"intVal"`
	StrVal string `json:"strVal"`
}

type Type int

const (
	Int    Type = iota // The IntOrString holds an int.
	String             // The IntOrString holds a string.
)

func (intstr *IntOrString) UnmarshalJSON(value []byte) error {
	if value[0] == '"' {
		intstr.Type = String
		return json.Unmarshal(value, &intstr.StrVal)
	}
	intstr.Type = Int
	return json.Unmarshal(value, &intstr.IntVal)
}

func (intstr IntOrString) MarshalJSON() ([]byte, error) {
	switch intstr.Type {
	case Int:
		return json.Marshal(intstr.IntVal)
	case String:
		return json.Marshal(intstr.StrVal)
	default:
		return []byte{}, fmt.Errorf("impossible IntOrString.Type")
	}
}
