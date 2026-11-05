import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { RootState } from '../store';
import { createRateLimits, deleteRateLimit, deleteRateLimitRelease, describeLimitRules, DescribeLimitRulesRequest, describeOneLimitRules, describeRateLimitVersions, DescribeRateLimitVersionsRequest, LimitType, modifyRateLimits, publishRateLimit, RateLimitView, rateLimitViewToRequest, rollbackRateLimit } from 'services/ratelimit';
import { RuleRelease } from 'services/types';
import { VersionClient } from 'services/version_client';

// 定义规则接口（大规则 + 子规则列表）
export interface RateLimitRuleState {
    datas: RateLimitView[]
    total: number
    page: number
    limit: number
    loading: boolean

    editRule: RateLimitView | null
    viewRule: RateLimitView | null

    versions: RuleRelease[]
    versionTotal: number
    versionPage: number
    versionLimit: number
    versionLoading: boolean

    subscribers?: VersionClient[]
}

const initialState: RateLimitRuleState = {
    datas: [],
    total: 0,
    page: 1,
    limit: 10,
    loading: false,

    editRule: null,
    viewRule: null,

    versions: [],
    versionTotal: 0,
    versionPage: 1,
    versionLimit: 10,
    versionLoading: false
};

// 创建限流规则
export const listOneRateLimitRule = createAsyncThunk(`ratelimit/list_one`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeOneLimitRules(id); // 创建时默认启用
        return fulfillWithValue({
            viewRule: res,
        });
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

// 创建限流规则
export const listRateLimitRules = createAsyncThunk(`ratelimit/list`, async ({ param }: { param: DescribeLimitRulesRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeLimitRules(param); // 创建时默认启用
        return fulfillWithValue({
            datas: res.list,
            total: res.totalCount,
            page: Math.floor(param.offset / param.limit) + 1,
            limit: param.limit || 10
        });
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

// 创建限流规则（新结构：大规则 + 子规则列表）
export const saveRateLimitRule = createAsyncThunk(`ratelimit/create`, async ({ param }: { param: RateLimitView }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const req = rateLimitViewToRequest(param);
        const res = await createRateLimits([req]);
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

// 更新限流规则（新结构）
export const updateRateLimitRule = createAsyncThunk(`ratelimit/update`, async ({ param }: { param: RateLimitView }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const req = rateLimitViewToRequest(param);
        const res = await modifyRateLimits([req]);
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

// 删除限流规则
export const removeRateLimitRule = createAsyncThunk(`ratelimit/remove`, async ({ ids }: { ids: string[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteRateLimit(ids.map((id) => ({ id })));
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

export const releaseRateLimitRule = createAsyncThunk(`ratelimit/release`, async ({ param }: { param: RuleRelease[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await publishRateLimit(param);
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

// 列出限流规则版本
export const listRateLimitRuleVersions = createAsyncThunk(`ratelimit/list_releases`, async ({ param }: { param: DescribeRateLimitVersionsRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeRateLimitVersions(param); // 创建时默认启用
        return fulfillWithValue({
            datas: res.list,
            total: res.totalCount,
            page: Math.floor(param.offset / param.limit) + 1,
            limit: param.limit || 10
        });
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

export const rollbackRateLimitRuleVersion = createAsyncThunk(`ratelimit/rollback_release`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await rollbackRateLimit({
            id: id
        });
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

export const removeRateLimitRuleVersion = createAsyncThunk(`ratelimit/delete_release`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteRateLimitRelease(
            {
                id: id,
            });
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

const rateLimitReducer = createSlice({
    name: 'ratelimit/edit',
    initialState,
    reducers: {
        editorRateLimitRule: (state, action: PayloadAction<RateLimitView>) => {
            state = {
                ...state,
                editRule: {
                    ...action.payload
                }
            };
            return state;
        },
        resetRateLimitRule: (state) => {
            state = {
                ...state,
                editRule: null,
                viewRule: null,
            };
            return state;
        },
        cleanRateLimitPage: (state) => {
            state = {
                ...state,
                datas: [],
                total: 0,
                page: 1,
                limit: 10,
                loading: false,
            };
            return state;
        },
        cleanRateLimitVersions: (state) => {
            state = {
                ...state,
                versions: [],
                versionTotal: 0,
                versionPage: 1,
                versionLimit: 10,
                versionLoading: false,
            };
            return state;
        }
    },
    extraReducers: (builder) => {
        builder
            .addCase(listRateLimitRules.fulfilled, (state, action) => {
                state.datas = action.payload.datas;
                state.total = action.payload.total;
                state.page = action.payload.page;
                state.limit = action.payload.limit;
                state.loading = false;
            })
            .addCase(listRateLimitRules.pending, (state) => {
                state.loading = true;
            })
            .addCase(listRateLimitRules.rejected, (state, action) => {
                state.loading = false;
            })
            .addCase(listOneRateLimitRule.fulfilled, (state, action) => {
                state.viewRule = action.payload.viewRule;
            })
            .addCase(listRateLimitRuleVersions.fulfilled, (state, action) => {
                state.versions = action.payload.datas;
                state.versionTotal = action.payload.total;
                state.versionPage = action.payload.page;
                state.versionLimit = action.payload.limit;
                state.versionLoading = false;
            })
            .addCase(listRateLimitRuleVersions.pending, (state) => {
                state.versionLoading = true;
            })
            .addCase(listRateLimitRuleVersions.rejected, (state, action) => {
                state.versionLoading = false;
            });
        }
});

export const {
    editorRateLimitRule,
    resetRateLimitRule,
    cleanRateLimitPage,
    cleanRateLimitVersions,
} = rateLimitReducer.actions;
export const selectRateLimitRule = (state: RootState) => state.rateLimit;

export default rateLimitReducer.reducer;
