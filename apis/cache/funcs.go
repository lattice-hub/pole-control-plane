/**
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package cache

import (
	"context"
)

//

type (
	namespacePredicateCtxKey          struct{}
	servicePredicateCtxKey            struct{}
	routeRulePredicateCtxKey          struct{}
	ratelimitRulePredicateCtxKey      struct{}
	circuitbreakerRulePredicateCtxKey struct{}
	faultdetectRulePredicateCtxKey    struct{}
	laneRulePredicateCtxKey           struct{}
	losslessRulePredicateCtxKey       struct{}
	trafficGovernancePredicateCtxKey  struct{}
	userPredicateCtxKey               struct{}
	userGroupPredicateCtxKey          struct{}
	authPolicyPredicateCtxKey         struct{}
	authRolePredicateCtxKey           struct{}
)

func AppendNamespacePredicate(ctx context.Context, p NamespacePredicate) context.Context {
	var predicates []NamespacePredicate

	val := ctx.Value(namespacePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]NamespacePredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, namespacePredicateCtxKey{}, predicates)
}

func LoadNamespacePredicates(ctx context.Context) []NamespacePredicate {
	var predicates []NamespacePredicate

	val := ctx.Value(namespacePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]NamespacePredicate)
	}
	return predicates
}

func AppendServicePredicate(ctx context.Context, p ServicePredicate) context.Context {
	var predicates []ServicePredicate

	val := ctx.Value(servicePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]ServicePredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, servicePredicateCtxKey{}, predicates)
}

func LoadServicePredicates(ctx context.Context) []ServicePredicate {
	var predicates []ServicePredicate

	val := ctx.Value(servicePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]ServicePredicate)
	}
	return predicates
}

func AppendRouterRulePredicate(ctx context.Context, p RouteRulePredicate) context.Context {
	var predicates []RouteRulePredicate

	val := ctx.Value(routeRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]RouteRulePredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, routeRulePredicateCtxKey{}, predicates)
}

func LoadRouterRulePredicates(ctx context.Context) []RouteRulePredicate {
	var predicates []RouteRulePredicate

	val := ctx.Value(routeRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]RouteRulePredicate)
	}
	return predicates
}

func AppendRatelimitRulePredicate(ctx context.Context, p RateLimitRulePredicate) context.Context {
	var predicates []RateLimitRulePredicate

	val := ctx.Value(ratelimitRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]RateLimitRulePredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, ratelimitRulePredicateCtxKey{}, predicates)
}

func LoadRatelimitRulePredicates(ctx context.Context) []RateLimitRulePredicate {
	var predicates []RateLimitRulePredicate

	val := ctx.Value(ratelimitRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]RateLimitRulePredicate)
	}
	return predicates
}

func AppendLosslessRulePredicate(ctx context.Context, p LosslessPredicate) context.Context {
	var predicates []LosslessPredicate

	val := ctx.Value(losslessRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]LosslessPredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, losslessRulePredicateCtxKey{}, predicates)
}

func LoadLosslessRulePredicates(ctx context.Context) []LosslessPredicate {
	var predicates []LosslessPredicate

	val := ctx.Value(ratelimitRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]LosslessPredicate)
	}
	return predicates
}

func AppendTrafficGovernancePredicate(ctx context.Context, p TrafficGovernancePredicate) context.Context {
	var predicates []TrafficGovernancePredicate

	val := ctx.Value(trafficGovernancePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]TrafficGovernancePredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, trafficGovernancePredicateCtxKey{}, predicates)
}

func LoadTrafficGovernancePredicates(ctx context.Context) []TrafficGovernancePredicate {
	var predicates []TrafficGovernancePredicate

	val := ctx.Value(trafficGovernancePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]TrafficGovernancePredicate)
	}
	return predicates
}

func AppendCircuitBreakerRulePredicate(ctx context.Context, p CircuitBreakerPredicate) context.Context {
	var predicates []CircuitBreakerPredicate

	val := ctx.Value(circuitbreakerRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]CircuitBreakerPredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, circuitbreakerRulePredicateCtxKey{}, predicates)
}

func LoadCircuitBreakerRulePredicates(ctx context.Context) []CircuitBreakerPredicate {
	var predicates []CircuitBreakerPredicate

	val := ctx.Value(circuitbreakerRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]CircuitBreakerPredicate)
	}
	return predicates
}

func AppendFaultDetectRulePredicate(ctx context.Context, p FaultDetectPredicate) context.Context {
	var predicates []FaultDetectPredicate

	val := ctx.Value(faultdetectRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]FaultDetectPredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, faultdetectRulePredicateCtxKey{}, predicates)
}

func LoadFaultDetectRulePredicates(ctx context.Context) []FaultDetectPredicate {
	var predicates []FaultDetectPredicate

	val := ctx.Value(faultdetectRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]FaultDetectPredicate)
	}
	return predicates
}

func AppendLaneRulePredicate(ctx context.Context, p LanePredicate) context.Context {
	var predicates []LanePredicate

	val := ctx.Value(laneRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]LanePredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, laneRulePredicateCtxKey{}, predicates)
}

func LoadLaneRulePredicates(ctx context.Context) []LanePredicate {
	var predicates []LanePredicate

	val := ctx.Value(laneRulePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]LanePredicate)
	}
	return predicates
}

func AppendUserPredicate(ctx context.Context, p UserPredicate) context.Context {
	var predicates []UserPredicate

	val := ctx.Value(userPredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]UserPredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, userPredicateCtxKey{}, predicates)
}

func LoadUserPredicates(ctx context.Context) []UserPredicate {
	var predicates []UserPredicate

	val := ctx.Value(userPredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]UserPredicate)
	}
	return predicates
}

func AppendUserGroupPredicate(ctx context.Context, p UserGroupPredicate) context.Context {
	var predicates []UserGroupPredicate

	val := ctx.Value(userGroupPredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]UserGroupPredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, userGroupPredicateCtxKey{}, predicates)
}

func LoadUserGroupPredicates(ctx context.Context) []UserGroupPredicate {
	var predicates []UserGroupPredicate

	val := ctx.Value(userGroupPredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]UserGroupPredicate)
	}
	return predicates
}

func AppendAuthPolicyPredicate(ctx context.Context, p AuthPolicyPredicate) context.Context {
	var predicates []AuthPolicyPredicate

	val := ctx.Value(authPolicyPredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]AuthPolicyPredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, authPolicyPredicateCtxKey{}, predicates)
}

func LoadAuthPolicyPredicates(ctx context.Context) []AuthPolicyPredicate {
	var predicates []AuthPolicyPredicate

	val := ctx.Value(authPolicyPredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]AuthPolicyPredicate)
	}
	return predicates
}

func AppendAuthRolePredicate(ctx context.Context, p AuthRolePredicate) context.Context {
	var predicates []AuthRolePredicate

	val := ctx.Value(authRolePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]AuthRolePredicate)
	}

	predicates = append(predicates, p)
	return context.WithValue(ctx, authRolePredicateCtxKey{}, predicates)
}

func LoadAuthRolePredicates(ctx context.Context) []AuthRolePredicate {
	var predicates []AuthRolePredicate

	val := ctx.Value(authRolePredicateCtxKey{})
	if val != nil {
		predicates, _ = val.([]AuthRolePredicate)
	}
	return predicates
}
