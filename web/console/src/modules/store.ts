import { configureStore, combineReducers } from '@reduxjs/toolkit';
import { TypedUseSelectorHook, useSelector, useDispatch } from 'react-redux';

import global from './global';
import userLogin from './user/login';
import userUsers from './user/users';
import userGroups from './user/groups';
import namespace from './namespace';
import discoveryService from './discovery/service';
import discoveryServiceAlais from './discovery/alias';
import discoveryInstance from './discovery/instance';
import customRoute from './governance/route';
import laneGroup from './governance/lane_group';
import laneRule from './governance/lane_rule';
import rateLimit from './governance/ratelimit';
import circuitBreaker from './governance/circuitbreaker';
import faultDetect from './governance/faultdetect';
import authPolicyRules from './auth/policy';
import authRoles from './auth/role';
import lossless from './governance/lossless';
import aiA2A from './ai/a2a';
import aiMCP from './ai/mcp';

const reducer = combineReducers({
  global,
  userLogin,
  userUsers,
  userGroups,
  namespace,
  discoveryService,
  discoveryServiceAlais,
  discoveryInstance,
  customRoute,
  lossless,
  laneGroup,
  laneRule,
  circuitBreaker,
  faultDetect,
  rateLimit,
  authPolicyRules,
  authRoles,
  aiA2A,
  aiMCP,
});

export const store = configureStore({
  reducer,
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;

export const useAppDispatch = () => useDispatch<AppDispatch>();
export const useAppSelector: TypedUseSelectorHook<RootState> = useSelector;

export default store;
