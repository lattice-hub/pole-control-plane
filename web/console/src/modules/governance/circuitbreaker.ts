import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { RootState } from '../store';
import { BlockConfig, BreakLevelType, CircuitBreakerRule, createCircuitBreaker, CreateCircuitBreakerRequest, deleteCircuitBreaker, deleteCircuitBreakerRelease, describeCircuitBreakers, DescribeCircuitBreakersRequest, describeCircuitBreakerVersions, DescribeCircuitBreakerVersionsRequest, describeOneCircuitBreaker, ErrorCondition, ErrorConditionType, FallbackConfig, FaultDetectConfig, modifyCircuitBreaker, ModifyCircuitBreakerRequest, publishCircuitBreaker, RecoverCondition, rollbackCircuitBreaker, TriggerCondition, TriggerType } from "services/circuitbreaker"
import { Label, MatchType, RuleRelease } from "services/types"
import { VersionClient } from 'services/version_client';

export interface CircuitBreakerState {
    datas: CircuitBreakerRule[]
    total: number
    page: number
    limit: number
    loading: boolean

    editRule: CircuitBreakerRule | null
    viewRule: CircuitBreakerRule | null

    versions: RuleRelease[]
    versionTotal: number
    versionPage: number
    versionLimit: number
    versionLoading: boolean

    subscribers?: VersionClient[]
}

export const defaultBlockConfig: (idx: number) => BlockConfig = (idx: number) => ({
    name: `策略-${idx}`,
    error_conditions: [
        {
            inputType: ErrorConditionType.RET_CODE,
            condition: {
                type: MatchType.EXACT,
                value: '',
            }
        }
    ],
    trigger_conditions: [
        {
            triggerType: TriggerType.ERROR_RATE,
            errorCount: 3,
            errorPercent: 50,
            interval: 30,
            minimumRequest: 5
        }
    ],
    max_ejection_percent: 100,
    recoverCondition: {
        sleepWindow: 60,
        consecutiveSuccess: 0
    },
    faultDetectConfig: {
        enable: false
    },
    fallbackConfig: {
        enable: false,
        response: {
            code: 500,
            headers: [],
            body: ''
        },
    }
});

const initialState: CircuitBreakerState = {
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
    versionLoading: false,
}

export const listOneCircuitBreaker = createAsyncThunk(`circuit_breaker/list_one`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeOneCircuitBreaker(id);
        return fulfillWithValue({
            viewRule: res ? res : null,
        });
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const listCircuitBreakers = createAsyncThunk(`circuit_breaker/list`, async ({ param }: { param: DescribeCircuitBreakersRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeCircuitBreakers(param);
        return fulfillWithValue({
            datas: res.list,
            total: res.totalCount,
            page: Math.floor(param.offset / param.limit) + 1,
            limit: param.limit || 10
        });
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const saveCircuitBreakers = createAsyncThunk(`circuit_breaker/create`, async ({ param }: { param: CreateCircuitBreakerRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await createCircuitBreaker([param]); // 创建自定义路由规则时，默认启用
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const updateCircuitBreakers = createAsyncThunk(`circuit_breaker/update`, async ({ param }: { param: ModifyCircuitBreakerRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await modifyCircuitBreaker([param]); // 创建自定义路由规则时，默认启用
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const removeCircuitBreakers = createAsyncThunk(`circuit_breaker/remove`, async ({ ids }: { ids: string[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteCircuitBreaker(ids.map((item) => ({ id: item })));
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const releaseCircuitBreaker = createAsyncThunk(`circuit_breaker/release`, async ({ param }: { param: RuleRelease[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await publishCircuitBreaker(param);
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

export const listCircuitBreakerVersions = createAsyncThunk(`circuit_breaker/list_releases`, async ({ param }: { param: DescribeCircuitBreakerVersionsRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeCircuitBreakerVersions(param);
        return fulfillWithValue({
            versions: res.list,
            versionTotal: res.totalCount,
            versionPage: Math.floor(param.offset / param.limit) + 1,
            versionLimit: param.limit || 10,
        });
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const rollbackCircuitBreakerRelease = createAsyncThunk(`circuit_breaker/rollback`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await rollbackCircuitBreaker({
            id: id
        });
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

export const removeCircuitBreakerRelease = createAsyncThunk(`circuit_breaker/delete_release`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteCircuitBreakerRelease({
            id: id
        });
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

const circuitBreakerReducer = createSlice({
    name: 'circuit_breaker/edit',
    initialState,
    reducers: {
        editorCircuitBreaker: (state, action: PayloadAction<CircuitBreakerRule>) => {
            state = {
                ...state,
                editRule: {
                    ...action.payload,
                }
            };
            return state;
        },
        resetCircuitBreaker: (state) => {
            state = {
                ...state,
                editRule: null,
                viewRule: null,
            };
            return state;
        },
        cleanCircuitBreakerPage: (state) => {
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
        cleanCircuitBreakerVersions: (state) => {
            state = {
                ...state,
                versions: [],
                versionTotal: 0,
                versionPage: 1,
                versionLimit: 10,
            };
            return state;
        }
    },
    extraReducers: (builder) => {
        builder
            .addCase(listCircuitBreakers.fulfilled, (state, action) => {
                state.datas = action.payload.datas;
                state.total = action.payload.total;
                state.page = action.payload.page;
                state.limit = action.payload.limit;
                state.loading = false;
            })
            .addCase(listCircuitBreakers.pending, (state) => {
                state.loading = true;
            })
            .addCase(listCircuitBreakers.rejected, (state, action) => {
                state.loading = false;
            })
            .addCase(listOneCircuitBreaker.fulfilled, (state, action) => {
                state.viewRule = action.payload.viewRule;
            })
            .addCase(listCircuitBreakerVersions.fulfilled, (state, action) => {
                state.versions = action.payload.versions;
                state.versionTotal = action.payload.versionTotal;
                state.versionPage = action.payload.versionPage;
                state.versionLimit = action.payload.versionLimit;
                state.versionLoading = false;
            })
            .addCase(listCircuitBreakerVersions.pending, (state) => {
                state.versionLoading = true;
            })
            .addCase(listCircuitBreakerVersions.rejected, (state, action) => {
                state.versionLoading = false;
            });
    },
})

export const {
    editorCircuitBreaker,
    resetCircuitBreaker,
    cleanCircuitBreakerPage,
    cleanCircuitBreakerVersions,
} = circuitBreakerReducer.actions;
export const selectCircuitBreaker = (state: RootState) => state.circuitBreaker;

export default circuitBreakerReducer.reducer;
