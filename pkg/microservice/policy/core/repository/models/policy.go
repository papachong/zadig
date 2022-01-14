/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package models

// Policy is a namespaced or cluster scoped, logical grouping of PolicyRules that can be referenced as a unit by a PolicyBinding.
// for a cluster scoped Policy, namespace is empty.
type Policy struct {
	Name      string  `bson:"name"      json:"name"`
	Namespace string  `bson:"namespace" json:"namespace"`
	Rules     []*Rule `bson:"rules"     json:"rules"`
}

func (Policy) TableName() string {
	return "policy"
}
