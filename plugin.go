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

package main

import (
	_ "github.com/pole-io/pole-server/pkg/admin/interceptor"
	_ "github.com/pole-io/pole-server/pkg/cache"
	_ "github.com/pole-io/pole-server/pkg/cache/auth"
	_ "github.com/pole-io/pole-server/pkg/cache/client"
	_ "github.com/pole-io/pole-server/pkg/cache/namespace"
	_ "github.com/pole-io/pole-server/pkg/cache/service"
	_ "github.com/pole-io/pole-server/pkg/goverrule/interceptor"
	_ "github.com/pole-io/pole-server/pkg/namespace/interceptor"
	_ "github.com/pole-io/pole-server/pkg/service/interceptor"
)
