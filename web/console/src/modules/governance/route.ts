import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { RootState } from '../store';
import { createCustomRoutes, CreateCustomRoutesRequest, CustomRoute, CustomRouteView, deleteCustomRoute, describeCustomRoutes, DescribeCustomRoutesRequest, describeCustomRouteVersions, DescribeCustomRouteVersionsRequest, describeOneCustomRoute, modifyCustomRoutes, ModifyCustomRoutesRequest, publishCustomRoute, RoutingConfig } from 'services/router';
import { RuleRelease } from 'services/types';
import { VersionClient } from 'services/version_client';


// State 和 Action 类型定义
export interface CustomRouteState {
    datas: CustomRoute[]
    total: number
    page: number
    limit: number
    loading: boolean

    editRoute: CustomRouteView | null
    viewRoute: CustomRouteView | null

    versions: RuleRelease[]
    versionTotal: number
    versionPage: number
    versionLimit: number
    versionLoading: boolean

    subscribers?: VersionClient[]
}

const initialState: CustomRouteState = {
    datas: [],
    total: 0,
    page: 1,
    limit: 10,
    loading: false,

    editRoute: null,
    viewRoute: null,

    versions: [],
    versionTotal: 0,
    versionPage: 1,
    versionLimit: 10,
    versionLoading: false
};

export const listOneCustomRoute = createAsyncThunk(`custom_route/list_one`, async ({ id }: { id : string }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeOneCustomRoute(id); // 创建自定义路由规则时，默认启用
        return fulfillWithValue({
            viewRoute: res,
        }); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const listCustomRoutes = createAsyncThunk(`custom_route/list`, async ({ param }: { param: DescribeCustomRoutesRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeCustomRoutes(param); // 创建自定义路由规则时，默认启用
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

export const saveCustomRoutes = createAsyncThunk(`custom_route/create`, async ({ param }: { param: CreateCustomRoutesRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await createCustomRoutes([{ ...param, enable: true }]); // 创建自定义路由规则时，默认启用
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const updateCustomRoutes = createAsyncThunk(`custom_route/update`, async ({ param }: { param: ModifyCustomRoutesRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await modifyCustomRoutes([{ ...param, enable: true }]); // 修改自定义路由规则时，默认启用
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const removeCustomRoutes = createAsyncThunk(`custom_route/remove`, async ({ ids }: { ids: string[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteCustomRoute(ids.map((item) => ({ id: item })));
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const releaseCustomRoutes = createAsyncThunk(`custom_route/release`, async ({ param }: { param: RuleRelease[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await publishCustomRoute(param);
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const listCustomRouteVersions = createAsyncThunk(`custom_route/list_releases`, async ({ param }: { param: DescribeCustomRouteVersionsRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeCustomRouteVersions(param);
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

const customRouteReducer = createSlice({
    name: 'custom_route/edit',
    initialState,
    reducers: {
        editorCustomRoute: (state, action: PayloadAction<CustomRouteView>) => {
            state = {
                ...state,
                editRoute: {
                    ...action.payload
                },
            };
            return state;
        },
        resetCustomRoute: (state) => {
            state = {
                ...state,
                editRoute: null,
                viewRoute: null,
            };
            return state;
        },
        cleanCustomRoutePage: (state) => {
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
        cleanCustomRouteVersions: (state) => {
            state = {
                ...state,
                versions: [],
                versionTotal: 0,
                versionPage: 1,
                versionLimit: 10,
                versionLoading: false,
            };
            return state;
        },
    },
    extraReducers: (builder) => {
        builder
            .addCase(listCustomRoutes.pending, (state) => {
                state.loading = true;
            })
            .addCase(listCustomRoutes.fulfilled, (state, action) => {
                state.loading = false;
                state.datas = action.payload.datas;
                state.total = action.payload.total;
                state.page = action.payload.page;
                state.limit = action.payload.limit;
            })
            .addCase(listCustomRoutes.rejected, (state) => {
                state.loading = false;
            })
            .addCase(listOneCustomRoute.fulfilled, (state, action) => {
                state.viewRoute = action.payload.viewRoute;
            })
            .addCase(listCustomRouteVersions.pending, (state) => {
                state.versionLoading = true;
            })
            .addCase(listCustomRouteVersions.fulfilled, (state, action) => {
                state.versionLoading = false;
                state.versions = action.payload.datas;
                state.versionTotal = action.payload.total;
                state.versionPage = action.payload.page;
                state.versionLimit = action.payload.limit;
            })
            .addCase(listCustomRouteVersions.rejected, (state) => {
                state.versionLoading = false;
            });
    },
})

export const {
    editorCustomRoute,
    resetCustomRoute,
    cleanCustomRoutePage,
    cleanCustomRouteVersions,
} = customRouteReducer.actions;
export const selectCustomRoute = (state: RootState) => state.customRoute;

export default customRouteReducer.reducer;
