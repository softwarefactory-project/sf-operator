// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package conds provides various utility functions regarding Status.Conditions for the sf-operator
package conds

import (
	"fmt"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
)

func GetOperatorConditionName() string {
	val, _ := utils.GetEnvVarValue("OPERATOR_CONDITION_NAME")
	return val
}

func mkCondition(conditiontype string, reason string, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditiontype,
		Status:             metav1.ConditionUnknown,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
}

// Function to add or update conditions of a resource
//
// If a condition of type t does not exist, it creates a new condition,
// otherwise updates with the new parameters

func RefreshCondition(conditions *[]metav1.Condition, conditiontype string, status metav1.ConditionStatus, reason string, message string) {
	foundCondition := metav1.Condition{}
	for i, condition := range *conditions {
		if condition.Type == conditiontype {
			foundCondition = condition
			if condition.Status != status ||
				condition.Reason != reason ||
				condition.Message != message {
				(*conditions)[i].Status = status
				(*conditions)[i].Reason = reason
				(*conditions)[i].Message = message
				(*conditions)[i].LastTransitionTime = metav1.NewTime(time.Now())
			}
		}
	}
	if reflect.DeepEqual(foundCondition, metav1.Condition{}) {
		*conditions = append([]metav1.Condition{mkCondition(conditiontype, reason, message)}, *conditions...)
	}
}

func UpdateConditions(conditions *[]metav1.Condition, condType string, ready bool) {
	var reason, message string
	var status metav1.ConditionStatus
	if ready {
		reason = "Complete"
		message = fmt.Sprintf("Initialization of %s service completed.", condType)
		status = metav1.ConditionTrue
	} else {
		reason = "Awaiting"
		message = fmt.Sprintf("Initializing %s service...", condType)
		status = metav1.ConditionFalse
	}
	RefreshCondition(conditions, condType, status, reason, message)
}
