import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { RootState } from '../store';
import { createLaneGroups, CreateLaneGroupsRequest, deleteLaneGroupReleases, deleteLaneGroups, describeLaneGroups, DescribeLaneGroupsRequest, describeLaneGroupVersions, DescribeLaneGroupVersionsRequest, LaneGroupView, modifyLaneGroups, ModifyLaneGroupsRequest, publishLaneGroups, rollbackLaneGroup } from "services/lane"
import { RuleRelease } from 'services/types';
import { VersionClient } from 'services/version_client';

export interface LaneGroupState {
    datas: LaneGroupView[]
    total: number
    page: number
    limit: number
    loading: boolean

    editGroup: LaneGroupView | null
    viewGroup: LaneGroupView | null

    versions: RuleRelease[]
    versionTotal: number
    versionPage: number
    versionLimit: number
    versionLoading: boolean

    subscribers?: VersionClient[]
}

const initialState: LaneGroupState = {
    datas: [],
    total: 0,
    page: 1,
    limit: 10,
    loading: false,

    editGroup: null,
    viewGroup: null,

    versions: [],
    versionTotal: 0,
    versionPage: 1,
    versionLimit: 10,
    versionLoading: false
};

export const listOneLaneGroup = createAsyncThunk(`lane_group/list_one`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeLaneGroups({
            id: id,
            offset: 0,
            limit: 1,
            brief: false
        });
        return fulfillWithValue({
            viewGroup: res.amount > 0 ? res.data[0] : null,
        }); // 返回 token
    } catch (error) {
        console.log('error', error);
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const listLaneGroups = createAsyncThunk(`lane_group/list`, async ({ param }: { param: DescribeLaneGroupsRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeLaneGroups(param);
        return fulfillWithValue({
            datas: res.data,
            total: res.amount,
            page: Math.floor(param.offset / param.limit) + 1,
            limit: param.limit || 10
        }); // 返回 token
    } catch (error) {
        console.log('error', error);
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const saveLaneGroups = createAsyncThunk(`lane_group/create`, async ({ param }: { param: CreateLaneGroupsRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await createLaneGroups([param]); // 创建自定义路由规则时，默认启用
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        console.log('error', error);
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const updateLaneGroups = createAsyncThunk(`lane_group/update`, async ({ param }: { param: ModifyLaneGroupsRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await modifyLaneGroups([param]); // 创建自定义路由规则时，默认启用
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        console.log('error', error);
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const removeLaneGroups = createAsyncThunk(`lane_group/remove`, async ({ ids }: { ids: string[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteLaneGroups(ids.map((item) => ({ id: item })));
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const releaseLaneGroups = createAsyncThunk(`lane_group/release`, async ({ param }: { param: RuleRelease[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await publishLaneGroups(param);
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const listLaneGroupVersions = createAsyncThunk(`lane_group/list_releases`, async ({ param }: { param: DescribeLaneGroupVersionsRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeLaneGroupVersions(param);
        return fulfillWithValue({
            datas: res.list,
            total: res.totalCount,
            page: Math.floor(param.offset / param.limit) + 1,
            limit: param.limit || 10
        }); // 返回 token
    } catch (error) {
        console.log('error', error);
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const rollbackLanGroupVersion = createAsyncThunk(`lane_group/rollback`, async ({ id }: { id: string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await rollbackLaneGroup([{
            id: id
        }]);
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const removeLaneGroupVersion = createAsyncThunk(`lane_group/remove_release`, async ({ ids }: { ids: string[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteLaneGroupReleases(ids.map((item) => ({ id: item })));
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

const laneGroupReducer = createSlice({
    name: 'lane_group/edit',
    initialState,
    reducers: {
        editorLaneGroup: (state, action: PayloadAction<LaneGroupView>) => {
            state = {
                ...state,
                editGroup: {
                    ...action.payload
                },
            };
            return state;
        },
        resetLaneGroup: (state) => {
            state = {
                ...state,
                editGroup: null,
            }
            return state;
        },
        cleanLaneGroupPage: (state) => {
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
        cleanLaneGroupVersions: (state) => {
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
            .addCase(listLaneGroups.fulfilled, (state, action) => {
                state.datas = action.payload.datas;
                state.total = action.payload.total;
                state.page = action.payload.page;
                state.limit = action.payload.limit;
                state.loading = false;
            })
            .addCase(listLaneGroups.pending, (state) => {
                state.loading = true;
            })
            .addCase(listLaneGroups.rejected, (state, action) => {
                state.loading = false;
            })
            .addCase(listOneLaneGroup.fulfilled, (state, action) => {
                state.viewGroup = action.payload.viewGroup;
            })
            .addCase(listLaneGroupVersions.fulfilled, (state, action) => {
                state.versions = action.payload.datas;
                state.versionTotal = action.payload.total;
                state.versionPage = action.payload.page;
                state.versionLimit = action.payload.limit;
                state.versionLoading = false;
            })
            .addCase(listLaneGroupVersions.pending, (state) => {
                state.versionLoading = true;
            })
            .addCase(listLaneGroupVersions.rejected, (state, action) => {
                state.versionLoading = false;
            });
    },
})

export const {
    editorLaneGroup,
    resetLaneGroup,
    cleanLaneGroupPage,
    cleanLaneGroupVersions,
} = laneGroupReducer.actions;
export const selectLaneGroup = (state: RootState) => state.laneGroup;

export default laneGroupReducer.reducer;
