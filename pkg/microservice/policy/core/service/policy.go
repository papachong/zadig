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

package service

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
)

type Policy struct {
	Name  string  `json:"name"`
	Rules []*Rule `json:"rules,omitempty"`
}

func CreatePolicy(ns string, policy *Policy, _ *zap.SugaredLogger) error {
	obj := &models.Policy{
		Name:      policy.Name,
		Namespace: ns,
	}

	for _, r := range policy.Rules {
		obj.Rules = append(obj.Rules, &models.Rule{
			Verbs:     r.Verbs,
			Kind:      r.Kind,
			Resources: r.Resources,
		})
	}

	return mongodb.NewPolicyColl().Create(obj)
}

func UpdatePolicy(ns string, policy *Policy, _ *zap.SugaredLogger) error {
	obj := &models.Policy{
		Name:      policy.Name,
		Namespace: ns,
	}

	for _, r := range policy.Rules {
		obj.Rules = append(obj.Rules, &models.Rule{
			Verbs:     r.Verbs,
			Kind:      r.Kind,
			Resources: r.Resources,
		})
	}
	return mongodb.NewPolicyColl().UpdatePolicy(obj)
}

func UpdateOrCreatePolicy(ns string, policy *Policy, _ *zap.SugaredLogger) error {
	obj := &models.Policy{
		Name:      policy.Name,
		Namespace: ns,
	}

	for _, r := range policy.Rules {
		obj.Rules = append(obj.Rules, &models.Rule{
			Verbs:     r.Verbs,
			Kind:      r.Kind,
			Resources: r.Resources,
		})
	}
	return mongodb.NewPolicyColl().UpdateOrCreate(obj)
}

func ListPolicies(projectName string, _ *zap.SugaredLogger) ([]*Policy, error) {
	var policies []*Policy
	projectPolicies, err := mongodb.NewPolicyColl().ListBy(projectName)
	if err != nil {
		return nil, err
	}
	for _, v := range projectPolicies {
		// frontend doesn't need to see contributor role
		if v.Name == string(setting.Contributor) {
			continue
		}
		policies = append(policies, &Policy{
			Name: v.Name,
		})
	}
	return policies, nil
}

func GetPolicy(ns, name string, _ *zap.SugaredLogger) (*Policy, error) {
	r, found, err := mongodb.NewPolicyColl().Get(ns, name)
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("policy %s not found", name)
	}

	res := &Policy{
		Name: r.Name,
	}
	for _, ru := range r.Rules {
		res.Rules = append(res.Rules, &Rule{
			Verbs:     ru.Verbs,
			Kind:      ru.Kind,
			Resources: ru.Resources,
		})
	}

	return res, nil
}

func DeletePolicy(name string, projectName string, logger *zap.SugaredLogger) error {
	err := mongodb.NewPolicyColl().Delete(name, projectName)
	if err != nil {
		logger.Errorf("Failed to delete policy %s in project %s, err: %s", name, projectName, err)
		return err
	}

	return mongodb.NewPolicyBindingColl().DeleteByPolicy(name, projectName)
}

func DeletePolicies(names []string, projectName string, logger *zap.SugaredLogger) error {
	if len(names) == 0 {
		return nil
	}
	if projectName == "" {
		return fmt.Errorf("projectName is empty")
	}

	if names[0] == "*" {
		names = []string{}
	}

	err := mongodb.NewPolicyColl().DeleteMany(names, projectName)
	if err != nil {
		logger.Errorf("Failed to delete policies %s in project %s, err: %s", names, projectName, err)
		return err
	}

	return mongodb.NewPolicyBindingColl().DeleteByPolicies(names, projectName)
}
