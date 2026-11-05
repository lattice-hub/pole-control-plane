import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { RootState } from '../store';
import { MatchString, MatchType, MatchValueType, RuleRelease } from 'services/types';
import { VersionClient } from 'services/version_client';
import { createLossLessRule, CreateLossLessRuleRequest, deleteLosslessRelease, deleteLossLessRule, describeLossLessRules, DescribeLossLessRulesRequest, describeLosslessVersions, DescribeLosslessVersionsRequest, describeOneLossLessRules, LossLessRuleView, modifyLossLessRule, ModifyLossLessRuleRequest, publishLossless, rollbackLossless } from 'services/lossless';
import { RateLimitRuleState } from './ratelimit';
import { deleteFaultDetectRelease } from 'services/faultdetect';

// 定义规则接口
export interface LosslessRuleState {
    datas: LossLessRuleView[]
    total: number
    page: number
    limit: number
    loading: boolean

    editRule: LossLessRuleView | null
    viewRule: LossLessRuleView | null

    versions: RuleRelease[]
    versionTotal: number
    versionPage: number
    versionLimit: number
    versionLoading: boolean

    subscribers?: VersionClient[]
}

const initialState: LosslessRuleState = {
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

// 创建无损规则
export const listOneLossLessRule = createAsyncThunk(`lossless/list_one`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeOneLossLessRules(id); // 创建时默认启用
        return fulfillWithValue({
            viewRule: res,
        });
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

// 创建无损规则
export const listLossLessRules = createAsyncThunk(`lossless/list`, async ({ param }: { param: DescribeLossLessRulesRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeLossLessRules(param); // 创建时默认启用
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

// 创建限流规则
export const saveLossLessRule = createAsyncThunk(`lossless/create`, async ({ param }: { param: CreateLossLessRuleRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await createLossLessRule([{
            ...param,
        }]); // 创建时默认启用
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
}
);

// 更新限流规则
export const updateLosslessRule = createAsyncThunk(`lossless/update`, async ({ param }: { param: ModifyLossLessRuleRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await modifyLossLessRule([{
            ...param,
        }]);
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
}
);

// 删除限流规则
export const removeLosslessRule = createAsyncThunk(`lossless/remove`, async ({ ids }: { ids: string[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteLossLessRule(ids.map((id) => ({ id })));
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

export const releaseLosslessRule = createAsyncThunk(`lossless/release`, async ({ param }: { param: RuleRelease[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await publishLossless(param);
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

// 列出无损规则版本
export const listLosslessRuleVersions = createAsyncThunk(`lossless/list_releases`, async ({ param }: { param: DescribeLosslessVersionsRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeLosslessVersions(param); // 创建时默认启用
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

export const rollbackLosslessVersion = createAsyncThunk(`lossless/rollback_release`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await rollbackLossless({
            id: id
        });
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

export const removeLosslessVersion = createAsyncThunk(`lossless/delete_release`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteLosslessRelease(
            {
                id: id,
            });
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

const losslessReducer = createSlice({
    name: 'lossless/edit',
    initialState,
    reducers: {
        editorLosslessRule: (state, action: PayloadAction<LossLessRuleView>) => {
            state = {
                ...state,
                editRule: {
                    ...action.payload
                }
            };
            return state;
        },
        resetLosslessRule: (state) => {
            state = {
                ...state,
                editRule: null,
                viewRule: null,
            };
            return state;
        },
        cleanLosslessPage: (state) => {
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
        cleanLosslessVersions: (state) => {
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
            .addCase(listLossLessRules.fulfilled, (state, action) => {
                state.datas = action.payload.datas;
                state.total = action.payload.total;
                state.page = action.payload.page;
                state.limit = action.payload.limit;
                state.loading = false;
            })
            .addCase(listLossLessRules.pending, (state) => {
                state.loading = true;
            })
            .addCase(listLossLessRules.rejected, (state, action) => {
                state.loading = false;
            })
            .addCase(listOneLossLessRule.fulfilled, (state, action) => {
                state.viewRule = action.payload.viewRule;
            })
            .addCase(listLosslessRuleVersions.fulfilled, (state, action) => {
                state.versions = action.payload.datas;
                state.versionTotal = action.payload.total;
                state.versionPage = action.payload.page;
                state.versionLimit = action.payload.limit;
                state.versionLoading = false;
            })
            .addCase(listLosslessRuleVersions.pending, (state) => {
                state.versionLoading = true;
            })
            .addCase(listLosslessRuleVersions.rejected, (state, action) => {
                state.versionLoading = false;
            });
        }
});

export const {
    editorLosslessRule,
    resetLosslessRule,
    cleanLosslessPage,
    cleanLosslessVersions,
} = losslessReducer.actions;
export const selectLosslessRule = (state: RootState) => state.lossless;

export default losslessReducer.reducer;
