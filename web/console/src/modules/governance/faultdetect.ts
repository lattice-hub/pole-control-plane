import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { RootState } from '../store';
import { CreateFaultDetectRequest, createFaultDetects, deleteFaultDetectRelease, deleteFaultDetects, describeFaultDetects, DescribeFaultDetectsRequest, describeFaultDetectVersions, DescribeFaultDetectVersionsRequest, FaultDetectRule, ModifyFaultDetectRequest, modifyFaultDetects, publishFaultDetect, rollbackFaultDetect } from 'services/faultdetect';
import { RuleRelease } from 'services/types';
import { VersionClient } from 'services/version_client';

export interface FaultDetectState {
    datas: FaultDetectRule[]
    total: number
    page: number
    limit: number
    loading: boolean

    editRule: FaultDetectRule | null
    viewRule: FaultDetectRule | null

    versions: RuleRelease[]
    versionTotal: number
    versionPage: number
    versionLimit: number
    versionLoading: boolean

    subscribers?: VersionClient[]
}

const initialState: FaultDetectState = {
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
}

export const listOneFaultDetect = createAsyncThunk(`fault_detect/list_one`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeFaultDetects({
            id: id,
            limit: 1,
            offset: 0,
            brief: false,
        });
        return fulfillWithValue({
            editRule: res.totalCount > 0 ? res.list[0] : null,
        }); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const listFaultDetects = createAsyncThunk(`fault_detect/list`, async ({ param }: { param: DescribeFaultDetectsRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeFaultDetects(param);
        return fulfillWithValue({
            datas: res.list,
            total: res.totalCount,
            page: Math.floor(param.offset / param.limit) + 1,
            limit: param.limit || 10
        }); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const saveFaultDetects = createAsyncThunk(`fault_detect/create`, async ({ param }: { param: CreateFaultDetectRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await createFaultDetects([param]); // 创建自定义路由规则时，默认启用
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const updateFaultDetects = createAsyncThunk(`fault_detect/update`, async ({ param }: { param: ModifyFaultDetectRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await modifyFaultDetects([param]); // 创建自定义路由规则时，默认启用
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const removeFaultDetects = createAsyncThunk(`fault_detect/remove`, async ({ ids }: { ids: string[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteFaultDetects(ids.map((item) => ({ id: item })));
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const releaseFaultDetect = createAsyncThunk(`fault_detect/release`, async ({ param }: { param: RuleRelease[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await publishFaultDetect(param);
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

export const listFaultDetectVersions = createAsyncThunk(`fault_detect/list_releases`, async ({ param }: { param: DescribeFaultDetectVersionsRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeFaultDetectVersions(param);
        return fulfillWithValue({
            versions: res.list,
            versionTotal: res.totalCount,
            versionPage: Math.floor(param.offset / param.limit) + 1,
            versionLimit: param.limit || 10
        }); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const rollbackFaultDetectVersion = createAsyncThunk(`fault_detect/rollback_release`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await rollbackFaultDetect({
            id: id
        });
        return fulfillWithValue(res);
    } catch (error) {
        return rejectWithValue((error as Error).message);
    }
});

export const removeFaultDetectVersion = createAsyncThunk(`fault_detect/delete_release`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteFaultDetectRelease(
            {
                id: id,
            });
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

const faultDetectReducer = createSlice({
    name: 'fault_detect/edit',
    initialState,
    reducers: {
        editorFaultDetect: (state, action: PayloadAction<FaultDetectRule>) => {
            state = {
                ...state,
                editRule: {
                    ...action.payload
                }
            };
            return state;
        },
        resetFaultDetect: (state) => {
            state = {
                ...state,
                editRule: null,
                viewRule: null,
            };
            return state;
        },
        clearFaultDetect: (state) => {
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
        cleanFaultDetectVersions: (state) => {
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
            .addCase(listFaultDetects.fulfilled, (state, action) => {
                state.datas = action.payload.datas;
                state.total = action.payload.total;
                state.page = action.payload.page;
                state.limit = action.payload.limit;
                state.loading = false;
            })
            .addCase(listFaultDetects.pending, (state) => {
                state.loading = true;
            })
            .addCase(listFaultDetects.rejected, (state, action) => {
                state.loading = false;
            })
            .addCase(listOneFaultDetect.fulfilled, (state, action) => {
                state.editRule = action.payload.editRule;
            })
            .addCase(listFaultDetectVersions.fulfilled, (state, action) => {
                state.versions = action.payload.versions;
                state.versionTotal = action.payload.versionTotal;
                state.versionPage = action.payload.versionPage;
                state.versionLimit = action.payload.versionLimit;
                state.versionLoading = false;
            })
            .addCase(listFaultDetectVersions.pending, (state) => {
                state.versionLoading = true;
            })
            .addCase(listFaultDetectVersions.rejected, (state, action) => {
                state.versionLoading = false;
            });
        },
})

export const {
    editorFaultDetect,
    resetFaultDetect,
    clearFaultDetect,
    cleanFaultDetectVersions,
} = faultDetectReducer.actions;
export const selectFaultDetect = (state: RootState) => state.faultDetect;

export default faultDetectReducer.reducer;
