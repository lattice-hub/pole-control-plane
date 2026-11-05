import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { RootState } from '../store';
import { deleteLaneRules, modifyLaneRules, createLaneRules, LaneGroupView, CreateLaneRulesRequest, ModifyLaneRulesRequest, LaneRuleView } from "services/lane"
import { VersionClient } from 'services/version_client';

export interface LaneRuleState {
    datas: LaneGroupView[]
    total: number
    page: number
    limit: number
    loading: boolean

    editRule: LaneRuleView | null
    viewRule: LaneRuleView | null

    subscribers?: VersionClient[]
}

export const defaultLaneRule: () => LaneRuleState = () => ({
    datas: [],
    total: 0,
    page: 1,
    limit: 10,
    loading: false,

    editRule: null,
    viewRule: null
})


const initialState: LaneRuleState = defaultLaneRule();

export const saveLaneRules = createAsyncThunk(`lane_rule/create`, async ({ param }: { param: CreateLaneRulesRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await createLaneRules([param]); // 创建自定义路由规则时，默认启用
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const updateLaneRules = createAsyncThunk(`lane_rule/update`, async ({ param }: { param: ModifyLaneRulesRequest }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await modifyLaneRules([param]); // 创建自定义路由规则时，默认启用
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

export const removeLaneRules = createAsyncThunk(`lane_rule/remove`, async ({ ids }: { ids: {id: string, groupName: string}[] }, { fulfillWithValue, rejectWithValue }) => {
    try {
        const res = await deleteLaneRules(ids.map((item) => ({ id: item.id, groupName: item.groupName })));
        return fulfillWithValue(res); // 返回 token
    } catch (error) {
        return rejectWithValue((error as Error).message); // 捕获错误并返回
    }
});

const laneRuleReducer = createSlice({
    name: 'lane_rule/edit',
    initialState,
    reducers: {
        editorLaneRule: (state, action: PayloadAction<LaneRuleView>) => {
            state = {
                ...state,
                editRule: {
                    ...action.payload
                }
            };
            return state;
        },
        viewLaneRule: (state, action: PayloadAction<LaneRuleView>) => {
            state = {
                ...state,
                viewRule: {
                    ...action.payload
                }
            };
            return state;
        },
        resetLaneRule: (state) => {
            state = {
                ...state,
                editRule: null,
                viewRule: null,
            }
            return state;
        }
    },
    extraReducers: (builder) => {
    },
})

export const {
    editorLaneRule,
    resetLaneRule,
    viewLaneRule,
} = laneRuleReducer.actions;
export const selectLaneRule = (state: RootState) => state.laneRule;

export default laneRuleReducer.reducer;
