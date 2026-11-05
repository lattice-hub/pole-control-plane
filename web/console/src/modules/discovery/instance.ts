import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { RootState } from '../store';
import { CreateInstanceRequest, DeleteInstancesRequest, DescribeInstancesRequest, HEALTH_CHECK_STRUCT, Instance, InstanceLocation, InstanceView, ModifyInstanceRequest, createInstances, deleteInstances, describeInstances, modifyInstances } from 'services/instance';

// State 和 Action 类型定义
export interface InstanceState {
    datas: InstanceView[]
    total: number
    page: number
    limit: number
    loading: boolean

    editIns: Instance | null
}

const initialState: InstanceState = {
    datas: [],
    total: 0,
    page: 1,
    limit: 10,
    loading: false,

    editIns: null
};


export const listInstances = createAsyncThunk(`instance/list`, async ({ param }: { param: DescribeInstancesRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await describeInstances(param)
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

export const saveInstances = createAsyncThunk(`instance/create`, async ({ param }: { param: CreateInstanceRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await createInstances([param])
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const updateInstances = createAsyncThunk(`instance/update`, async ({ param }: { param: ModifyInstanceRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await modifyInstances([param]);
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const removeInstances = createAsyncThunk(`instance/remove`, async ({ instances }: { instances: DeleteInstancesRequest[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteInstances(instances);
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});


const instanceReducer = createSlice({
    name: 'instance',
    initialState,
    reducers: {
        editorInstance: (state, action: PayloadAction<Instance>) => {
            state = {
                ...state,
                editIns: {
                    ...action.payload,
                },
            };
            return state;
        },
        resetInstance: (state) => {
            state = {
                ...state,
                editIns: null
            };
            return state;
        },
        cleanInsPage: (state) => {
            state = {
                ...state,
                datas: [],
                total: 0,
                page: 1,
                limit: 10,
                loading: false,
            };
            return state;
        }
    },
    extraReducers: (builder) => {
        builder
            .addCase(listInstances.pending, (state) => {
                state.loading = true;
            })
            .addCase(listInstances.fulfilled, (state, action) => {
                state.datas = action.payload.datas;
                state.total = action.payload.total;
                state.page = action.payload.page;
                state.limit = action.payload.limit;
                state.loading = false;
            })
            .addCase(listInstances.rejected, (state) => {
                state.loading = false;
            });
    }
})


export const {
    editorInstance,
    resetInstance,
    cleanInsPage,
} = instanceReducer.actions;
export const selectInstance = (state: RootState) => state.discoveryInstance;

export default instanceReducer.reducer;
