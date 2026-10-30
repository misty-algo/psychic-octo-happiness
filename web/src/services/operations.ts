import { request } from '@umijs/max';

import type { Envelope, ListParams } from '@/services/admin';

// ===== 提现审批（司机资金）=====

export type WithdrawalRow = {
  id: string;
  driver_id: string;
  amount_fen: number;
  status: string;
  dual_approval: boolean;
  approvals: number;
  reason: string;
  created_at: string;
  updated_at: string;
  driver_no?: string;
  driver_name?: string;
};

export type WithdrawalListParams = ListParams & {
  state?: string;
  keyword?: string;
};

export type WithdrawalResult = {
  withdrawal_id: string;
  status: string;
  paid: boolean;
  message: string;
};

export async function listWithdrawals(
  params?: WithdrawalListParams,
): Promise<Envelope<WithdrawalRow[]>> {
  return request<Envelope<WithdrawalRow[]>>('/api/ops/withdrawals', { params });
}

export async function approveWithdrawal(
  id: string,
  body?: { reason?: string },
): Promise<WithdrawalResult> {
  const response = await request<Envelope<WithdrawalResult>>(
    `/api/ops/withdrawals/${id}/approve`,
    { method: 'POST', data: body ?? {} },
  );
  return response.data;
}

export async function rejectWithdrawal(
  id: string,
  body: { reason: string },
): Promise<WithdrawalResult> {
  const response = await request<Envelope<WithdrawalResult>>(
    `/api/ops/withdrawals/${id}/reject`,
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function payWithdrawal(id: string): Promise<WithdrawalResult> {
  const response = await request<Envelope<WithdrawalResult>>(
    `/api/ops/withdrawals/${id}/pay`,
    { method: 'POST' },
  );
  return response.data;
}

// ===== 运营用户管理（A01）=====

export type OpsRestriction = {
  id: string;
  account_id: string;
  type: string;
  reason: string;
  scope: string;
  state: string;
  operator: string;
  created_at: string;
  revoked_at?: string;
  revoked_by?: string;
  revoke_reason?: string;
};

export type OpsUserItem = {
  id: string;
  phone_mask: string;
  real_name: string;
  role: 'pax' | 'drv';
  status: number;
  restrict: OpsRestriction[];
  created_at: string;
};

export type OpsUserDetail = OpsUserItem & {
  phone: string;
  passenger: boolean;
  driver_id: string;
};

export type OpsUserListParams = ListParams & {
  role?: string;
  keyword?: string;
};

export async function listOpsUsers(
  params?: OpsUserListParams,
): Promise<Envelope<OpsUserItem[]>> {
  return request<Envelope<OpsUserItem[]>>('/api/ops/users', { params });
}

export async function readOpsUser(id: string): Promise<OpsUserDetail> {
  const response = await request<Envelope<OpsUserDetail>>(
    `/api/ops/users/${id}`,
  );
  return response.data;
}

export async function restrictOpsUser(
  id: string,
  body: { type: string; reason: string; scope: string },
): Promise<OpsRestriction> {
  const response = await request<Envelope<OpsRestriction>>(
    `/api/ops/users/${id}/restrict`,
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function unrestrictOpsUser(
  id: string,
  body: { restriction_id: string; reason: string },
): Promise<OpsRestriction> {
  const response = await request<Envelope<OpsRestriction>>(
    `/api/ops/users/${id}/unrestrict`,
    { method: 'POST', data: body },
  );
  return response.data;
}

// ===== 司机车辆审核（A02）=====

export type AuditVehicle = {
  plate: string;
  model: string;
  seats: number;
  cert_no: string;
  cert_expire: string;
};

export type AuditItem = {
  application_id: string;
  revision: number;
  driver_id: string;
  driver_name: string;
  plate: string;
  submitter: string;
  state: '待审' | '通过' | '驳回';
  vehicle: AuditVehicle;
  claimed_by: string;
  claimed_at?: string;
  decided_by: string;
  decided_at?: string;
  reason: string;
  version: number;
  submitted_at: string;
  rejections: number;
  manual_review: boolean;
};

export type AuditHistory = {
  application_id: string;
  rejections: number;
  manual_review: boolean;
  revisions: AuditItem[];
};

export type AuditListParams = ListParams & {
  state?: string;
  keyword?: string;
};

export async function listDriverAudits(
  params?: AuditListParams,
): Promise<Envelope<AuditItem[]>> {
  return request<Envelope<AuditItem[]>>('/api/ops/driver-audits', { params });
}

export async function driverAuditHistory(
  applicationId: string,
): Promise<AuditHistory> {
  const response = await request<Envelope<AuditHistory>>(
    `/api/ops/driver-audits/${applicationId}`,
  );
  return response.data;
}

export async function claimDriverAudit(
  applicationId: string,
  revision: number,
): Promise<AuditItem> {
  const response = await request<Envelope<AuditItem>>(
    `/api/ops/driver-audits/${applicationId}/revisions/${revision}/claim`,
    { method: 'POST' },
  );
  return response.data;
}

export async function decideDriverAudit(
  applicationId: string,
  revision: number,
  body: { action: 'approve' | 'reject'; reason?: string; version: number },
): Promise<AuditItem> {
  const response = await request<Envelope<AuditItem>>(
    `/api/ops/driver-audits/${applicationId}/revisions/${revision}/decide`,
    { method: 'POST', data: body },
  );
  return response.data;
}

// ===== 订单全量管理（A03）=====

export type OrderRow = {
  id: string;
  order_no: string;
  biz_type: string;
  city_code: string;
  passenger_id: string;
  passenger_name: string;
  passenger_phone: string;
  driver_id: string;
  driver_name: string;
  plate: string;
  state: string;
  pay_status: string;
  amount: number;
  cancel_fee: number;
  start_address: string;
  end_address: string;
  risk_level: string;
  created_at: string;
};

export type OrderTask = {
  task_no: string;
  driver_id: string;
  driver_name: string;
  round: number;
  state: string;
  created_at: string;
};

export type OrderAttempt = {
  driver_id: string;
  driver_name: string;
  result: string;
  score: number;
  distance: number;
  responded_at: string;
};

export type OrderTrajectory = {
  source: string;
  points: Record<string, unknown>[];
};

export type OrderPricing = {
  version: string;
  rule_id: string;
  snapshot_id: string;
  estimate_fee: number;
  surge_ratio: number;
  total_fee: number;
  reviewed: boolean;
  reviewed_at: string;
  reviewed_by: string;
};

export type OrderPayment = {
  channel: string;
  trade_no: string;
  state: string;
  amount: number;
  refunded: boolean;
  refund_amount: number;
  raw: Record<string, unknown>;
};

export type OrderTicket = {
  ticket_no: string;
  type: string;
  state: string;
  created_at: string;
};

export type OrderDetailData = {
  order: OrderRow & {
    pricing_version: string;
    cancel_side: string;
    safety_flags: string[];
  };
  tasks: OrderTask[];
  attempts: OrderAttempt[];
  trajectory: OrderTrajectory;
  pricing: OrderPricing;
  payment?: OrderPayment;
  tickets: OrderTicket[];
};

export type OrderCommandResult = {
  order_id: string;
  order_no: string;
  command: string;
  state: string;
  basis: string;
};

export type OrderListParams = ListParams & {
  keyword?: string;
  state?: string;
  biz_type?: string;
  city_code?: string;
  risk_level?: string;
  from?: string;
  to?: string;
};

export async function listOrders(
  params?: OrderListParams,
): Promise<Envelope<OrderRow[]>> {
  return request<Envelope<OrderRow[]>>('/api/ops/orders', { params });
}

export async function readOrder(orderNo: string): Promise<OrderDetailData> {
  const response = await request<Envelope<OrderDetailData>>(
    `/api/ops/orders/${orderNo}`,
  );
  return response.data;
}

export async function executeOrderCommand(
  orderNo: string,
  body: { command: string; basis: string },
): Promise<OrderCommandResult> {
  const response = await request<Envelope<OrderCommandResult>>(
    `/api/ops/orders/${orderNo}/commands`,
    { method: 'POST', data: body },
  );
  return response.data;
}

// ===== 投诉工单处理（A04）=====

export type TicketRound = {
  round: number;
  handler: string;
  state: string;
  started_at: string;
  ended_at?: string;
  result: string;
};

export type TicketTimelineEntry = {
  seq: number;
  actor: string;
  action: string;
  from: string;
  to: string;
  message: string;
  at: string;
};

export type TicketFalseReport = {
  revoked: boolean;
  by: string;
  at: string;
  reason: string;
  driver_status_restored: boolean;
};

export type Ticket = {
  id: string;
  ticket_no: string;
  order_id: string;
  order_no: string;
  type: string;
  group: string;
  city_code: string;
  complainant_id: string;
  passenger_name: string;
  passenger_phone: string;
  driver_id: string;
  driver_name: string;
  plate: string;
  content: string;
  evidence_count: number;
  state: string;
  urgent: boolean;
  assignee: string;
  round: number;
  rounds: TicketRound[];
  created_at: string;
  first_response_due_at: string;
  first_responded_at?: string;
  urgent_response_due_at?: string;
  paused_ms: number;
  paused_at?: string;
  escalated: boolean;
  escalated_at?: string;
  escalate_reason: string;
  duty_reminded_at?: string;
  merged_ticket_nos: string[];
  merged_into: string;
  false_report?: TicketFalseReport;
  timeline: TicketTimelineEntry[];
};

export type TicketRow = {
  ticket_no: string;
  order_no: string;
  type: string;
  group: string;
  city_code: string;
  passenger_name: string;
  passenger_phone: string;
  driver_name: string;
  plate: string;
  state: string;
  urgent: boolean;
  assignee: string;
  round: number;
  created_at: string;
  first_response_due_at: string;
  remaining_hours: number;
  overdue: boolean;
  paused: boolean;
  escalated: boolean;
  rank: number;
};

export type TicketDetailData = {
  ticket: Ticket;
  duplicates: Ticket[];
};

export type TicketCommandResult = {
  ticket_no: string;
  command: string;
  state: string;
  round: number;
  assignee: string;
  first_response_due_at: string;
};

export type TicketListParams = ListParams & {
  keyword?: string;
  state?: string;
  type?: string;
  group?: string;
  overdue?: boolean;
};

export async function listTickets(
  params?: TicketListParams,
): Promise<Envelope<TicketRow[]>> {
  return request<Envelope<TicketRow[]>>('/api/ops/tickets', { params });
}

export async function readTicket(ticketNo: string): Promise<TicketDetailData> {
  const response = await request<Envelope<TicketDetailData>>(
    `/api/ops/tickets/${ticketNo}`,
  );
  return response.data;
}

export async function executeTicketCommand(
  ticketNo: string,
  body: { command: string; message: string; assignee?: string; merge_no?: string },
): Promise<TicketCommandResult> {
  const response = await request<Envelope<TicketCommandResult>>(
    `/api/ops/tickets/${ticketNo}/commands`,
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function appealTicket(
  ticketNo: string,
  body: { message: string },
): Promise<TicketCommandResult> {
  const response = await request<Envelope<TicketCommandResult>>(
    `/api/ops/tickets/${ticketNo}/appeal`,
    { method: 'POST', data: body },
  );
  return response.data;
}

// ===== 营销中心：优惠券与活动（A05）=====

export type CouponTemplate = {
  template_no: string;
  name: string;
  coupon_type: string;
  face_value_fen: number;
  threshold_fen: number;
  discount_rate: number;
  max_discount_fen: number;
  bearer: string;
  city_codes: string[];
  product_codes: string[];
  valid_start: string;
  valid_end: string;
  stock: number;
  issued_count: number;
  used_count: number;
  per_limit: number;
  stackable: boolean;
  mutual_templates: string[];
  description: string;
  state: string;
};

export type CouponBatch = {
  batch_no: string;
  template_no: string;
  template_name: string;
  total_count: number;
  issued_count: number;
  used_count: number;
  channel: string;
  budget_fen: number;
  used_budget_fen: number;
  status: string;
  start_at: string;
  end_at: string;
  campaign_code: string;
  operator: string;
};

export type IssuedCoupon = {
  coupon_no: string;
  template_no: string;
  template_name: string;
  batch_no: string;
  passenger_id: string;
  passenger_name: string;
  phone_mask: string;
  state: string;
  face_value_fen: number;
  threshold_fen: number;
  bearer: string;
  issued_at: string;
  expire_at: string;
  channel: string;
};

export type SubsidyConfig = {
  trigger_type: 'per_order' | 'time_period' | 'task';
  amount_fen: number;
  max_per_driver: number;
  daily_cap_fen: number;
  task_type?: string;
  need_enroll: boolean;
};

export type Campaign = {
  code: string;
  name: string;
  type: string;
  target_audience: string;
  rules: string;
  coupon_templates: string[];
  reward_amount_fen: number;
  subsidy_config?: SubsidyConfig | null;
  city_codes: string[];
  start_at: string;
  end_at: string;
  status: string;
  participant_count: number;
  budget_fen: number;
  used_budget_fen: number;
};

export type SettlementBatch = {
  batch_no: string;
  template_no: string;
  template_name: string;
  channel: string;
  total_count: number;
  issued_count: number;
  used_count: number;
  unused_count: number;
  face_value_fen: number;
  issue_amount_fen: number;
  used_amount_fen: number;
  bearer: string;
  status: string;
};

export type CampaignSettlement = {
  campaign_code: string;
  campaign_name: string;
  status: string;
  start_at: string;
  end_at: string;
  batches: SettlementBatch[];
  issued_count: number;
  used_count: number;
  unused_count: number;
  expired_count: number;
  issue_amount_fen: number;
  used_amount_fen: number;
  subsidy_granted_fen: number;
  subsidy_record_count: number;
  bearer_split: Record<string, number>;
  budget_fen: number;
  used_budget_fen: number;
  budget_usage_pct: number;
  closed: boolean;
};

export type SubsidyRecord = {
  record_no: string;
  campaign_code: string;
  campaign_name: string;
  driver_id: string;
  order_no?: string;
  amount_fen: number;
  biz_type: string;
  reason: string;
  day: string;
  operator: string;
  created_at: string;
};

export type CampaignSubsidySummary = {
  campaign_code: string;
  campaign_name: string;
  status: string;
  trigger_type: string;
  amount_fen: number;
  max_per_driver: number;
  daily_cap_fen: number;
  budget_fen: number;
  used_budget_fen: number;
  budget_remain_fen: number;
  granted_total_fen: number;
  record_count: number;
  driver_count: number;
  today_granted_fen: number;
  today: string;
};

export type IssueSkip = {
  passenger_id: string;
  phone: string;
  reason: string;
};

export type IssueResult = {
  batch_no: string;
  template_no: string;
  issued: number;
  skipped: IssueSkip[];
  coupon_nos: string[];
  remaining: number;
  degraded: boolean;
};

export type MarketingListParams = ListParams & {
  keyword?: string;
  state?: string;
  status?: string;
  type?: string;
  channel?: string;
  template_no?: string;
  batch_no?: string;
  passenger_id?: string;
  phone?: string;
  campaign_code?: string;
};

export async function listCouponTemplates(
  params?: MarketingListParams,
): Promise<Envelope<CouponTemplate[]>> {
  return request<Envelope<CouponTemplate[]>>('/api/ops/marketing/coupon-templates', {
    params,
  });
}

export async function createCouponTemplate(
  body: Record<string, unknown>,
): Promise<CouponTemplate> {
  const response = await request<Envelope<CouponTemplate>>(
    '/api/ops/marketing/coupon-templates',
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function executeTemplateCommand(
  templateNo: string,
  body: { command: string },
): Promise<CouponTemplate> {
  const response = await request<Envelope<CouponTemplate>>(
    `/api/ops/marketing/coupon-templates/${templateNo}/commands`,
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function listCouponBatches(
  params?: MarketingListParams,
): Promise<Envelope<CouponBatch[]>> {
  return request<Envelope<CouponBatch[]>>('/api/ops/marketing/coupon-batches', {
    params,
  });
}

export async function createCouponBatch(
  body: Record<string, unknown>,
): Promise<CouponBatch> {
  const response = await request<Envelope<CouponBatch>>(
    '/api/ops/marketing/coupon-batches',
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function issueCoupons(
  batchNo: string,
  body: { passenger_ids: string[] },
): Promise<IssueResult> {
  const response = await request<Envelope<IssueResult>>(
    `/api/ops/marketing/coupon-batches/${batchNo}/issue`,
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function executeBatchCommand(
  batchNo: string,
  body: { command: string },
): Promise<CouponBatch> {
  const response = await request<Envelope<CouponBatch>>(
    `/api/ops/marketing/coupon-batches/${batchNo}/commands`,
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function listIssuedCoupons(
  params?: MarketingListParams,
): Promise<Envelope<IssuedCoupon[]>> {
  return request<Envelope<IssuedCoupon[]>>('/api/ops/marketing/coupons', {
    params,
  });
}

export async function listCampaigns(
  params?: MarketingListParams,
): Promise<Envelope<Campaign[]>> {
  return request<Envelope<Campaign[]>>('/api/ops/marketing/campaigns', {
    params,
  });
}

export async function createCampaign(
  body: Record<string, unknown>,
): Promise<Campaign> {
  const response = await request<Envelope<Campaign>>(
    '/api/ops/marketing/campaigns',
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function executeCampaignCommand(
  code: string,
  body: { command: string },
): Promise<Campaign> {
  const response = await request<Envelope<Campaign>>(
    `/api/ops/marketing/campaigns/${code}/commands`,
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function readCampaignSettlement(
  code: string,
): Promise<CampaignSettlement> {
  const response = await request<Envelope<CampaignSettlement>>(
    `/api/ops/marketing/campaigns/${code}/settlement`,
  );
  return response.data;
}

// A08 补贴台账：登记发放、台账分页、补贴口径汇总。
export async function grantSubsidy(
  code: string,
  body: { driver_id: string; order_no?: string; biz_type: string; reason: string },
): Promise<{ record: SubsidyRecord; budget_remain_fen: number }> {
  const response = await request<Envelope<{ record: SubsidyRecord; budget_remain_fen: number }>>(
    `/api/ops/marketing/campaigns/${code}/subsidies`,
    { method: 'POST', data: body },
  );
  return response.data;
}

export async function listSubsidyRecords(
  code: string,
  params?: MarketingListParams & { driver_id?: string; biz_type?: string; day?: string },
): Promise<Envelope<SubsidyRecord[]>> {
  return request<Envelope<SubsidyRecord[]>>(
    `/api/ops/marketing/campaigns/${code}/subsidies`,
    { params },
  );
}

export async function readSubsidySummary(
  code: string,
): Promise<CampaignSubsidySummary> {
  const response = await request<Envelope<CampaignSubsidySummary>>(
    `/api/ops/marketing/campaigns/${code}/subsidy-summary`,
  );
  return response.data;
}

// A08 补贴追回：虚假订单或全额退款时冲正，预算同步退回。
export async function reverseSubsidy(
  code: string,
  recordNo: string,
  body: { reason: string },
): Promise<SubsidyRecord> {
  const response = await request<Envelope<SubsidyRecord>>(
    `/api/ops/marketing/campaigns/${code}/subsidies/${recordNo}/reverse`,
    { method: 'POST', data: body },
  );
  return response.data;
}

// A07 券作废：不可恢复，前端必须二次确认。
export async function voidCoupon(
  couponNo: string,
  body: { reason: string; confirm: boolean },
): Promise<IssuedCoupon> {
  const response = await request<Envelope<IssuedCoupon>>(
    `/api/ops/marketing/coupons/${couponNo}/void`,
    { method: 'POST', data: body },
  );
  return response.data;
}

// ==================== A05 计价规则 ====================

export type PricingRule = {
  rule_id: string;
  version: number;
  city_code: string;
  area?: string;
  car_type: string;
  start_fen: number;
  include_m: number;
  per_km_fen: number;
  per_min_fen: number;
  service_bp: number;
  comm_bp: number;
  surge_bp: number;
  discount_bp: number;
  cancel_fen: number;
  fixed_fen: number;
  priority: number;
  valid_from: string;
  valid_to: string;
  state: string;
  operator?: string;
  created_at: string;
  updated_at: string;
};

export type FareInput = {
  name?: string;
  dist_m: number;
  dur_s: number;
  coupon_fen?: number;
  subsidy_fen?: number;
  pooled?: boolean;
};

export type FareBreakdown = {
  t_fen: number;
  q_fen: number;
  h_fen: number;
  k_fen: number;
  f_fen: number;
  p_fen: number;
  d_fen: number;
  n_fen: number;
  g_fen: number;
};

export type FareSample = {
  name: string;
  input: FareInput;
  fare: FareBreakdown;
};

export async function listPricingRules(
  params?: MarketingListParams & { city_code?: string; car_type?: string; state?: string; keyword?: string },
): Promise<Envelope<PricingRule[]>> {
  return request<Envelope<PricingRule[]>>('/api/ops/pricing/rules', { params });
}

export async function createPricingRule(body: Partial<PricingRule>): Promise<PricingRule> {
  const response = await request<Envelope<PricingRule>>('/api/ops/pricing/rules', {
    method: 'POST',
    data: body,
  });
  return response.data;
}

export async function publishPricingRule(ruleId: string): Promise<PricingRule> {
  const response = await request<Envelope<PricingRule>>(
    `/api/ops/pricing/rules/${ruleId}/publish`,
    { method: 'POST' },
  );
  return response.data;
}

export async function disablePricingRule(ruleId: string): Promise<PricingRule> {
  const response = await request<Envelope<PricingRule>>(
    `/api/ops/pricing/rules/${ruleId}/disable`,
    { method: 'POST' },
  );
  return response.data;
}

export async function previewPricingFare(body: {
  rule_id?: string;
  rule?: Partial<PricingRule>;
  samples?: FareInput[];
}): Promise<FareSample[]> {
  const response = await request<Envelope<FareSample[]>>('/api/ops/pricing/preview', {
    method: 'POST',
    data: body,
  });
  return response.data;
}

export async function readEffectivePricingRule(
  cityCode: string,
  carType: string,
  at?: string,
): Promise<PricingRule> {
  const response = await request<Envelope<PricingRule>>('/api/ops/pricing/rules/effective', {
    params: { city_code: cityCode, car_type: carType, at },
  });
  return response.data;
}

// ==================== A06 派单策略 ====================

export type DispatchWeights = {
  income_bp: number;
  distance_bp: number;
  rating_bp: number;
  online_sec_bp: number;
  success_rate_bp: number;
};

export type DispatchStrategy = {
  strategy_id: string;
  version: number;
  weights: DispatchWeights;
  radius_m: number;
  invite_sec: number;
  total_sec: number;
  listen_sec: number;
  grab_sec: number;
  region_switch: boolean;
  redispatch: { extra_window_sec: number; max_accepted_return: number };
  state: string;
  operator?: string;
  created_at: string;
  updated_at: string;
  published_at?: string;
};

export type RankedCandidate = {
  rank: number;
  score: number;
  detail: Record<string, number>;
  driver: {
    driver_id: string;
    driver_name?: string;
    income_fen: number;
    distance_m: number;
    rating: number;
    online_sec: number;
    success_rate: number;
  };
};

export async function listDispatchStrategies(
  params?: MarketingListParams & { state?: string },
): Promise<Envelope<DispatchStrategy[]>> {
  return request<Envelope<DispatchStrategy[]>>('/api/ops/dispatch/strategies', { params });
}

export async function createDispatchStrategy(body: {
  income_bp: number;
  distance_bp: number;
  rating_bp: number;
  online_sec_bp: number;
  success_rate_bp: number;
  radius_m: number;
  invite_sec: number;
  total_sec: number;
  listen_sec: number;
  grab_sec: number;
  region_switch: boolean;
  extra_window_sec: number;
  max_accepted_return: number;
}): Promise<DispatchStrategy> {
  const response = await request<Envelope<DispatchStrategy>>('/api/ops/dispatch/strategies', {
    method: 'POST',
    data: body,
  });
  return response.data;
}

export async function publishDispatchStrategy(strategyId: string): Promise<DispatchStrategy> {
  const response = await request<Envelope<DispatchStrategy>>(
    `/api/ops/dispatch/strategies/${strategyId}/publish`,
    { method: 'POST' },
  );
  return response.data;
}

export async function previewDispatchRanking(
  strategyId: string,
  candidates?: unknown[],
): Promise<RankedCandidate[]> {
  const response = await request<Envelope<RankedCandidate[]>>(
    `/api/ops/dispatch/strategies/${strategyId}/preview`,
    { method: 'POST', data: { candidates } },
  );
  return response.data;
}

// ==================== A09 拼车与特价 ====================

export type PoolConfig = {
  config_id: string;
  version: number;
  match_sec: number;
  group_max: number;
  seat_max: number;
  city_codes?: string[];
  state: string;
  operator?: string;
  created_at: string;
  updated_at: string;
};

export type SpecialPricePlan = {
  plan_id: string;
  name: string;
  city_code: string;
  car_type: string;
  fixed_fen: number;
  seats: number;
  valid_from: string;
  valid_to: string;
  state: string;
  operator?: string;
  created_at: string;
};

export type PoolStats = {
  pool_order_count: number;
  grouped_count: number;
  matched_count: number;
  shared_count: number;
  unmatched_count: number;
  match_success_rate: number;
  shared_rate: number;
};

export async function listPoolConfigs(
  params?: MarketingListParams & { state?: string },
): Promise<Envelope<PoolConfig[]>> {
  return request<Envelope<PoolConfig[]>>('/api/ops/pool/configs', { params });
}

export async function createPoolConfig(body: {
  match_sec: number;
  group_max: number;
  seat_max: number;
  city_codes?: string[];
}): Promise<PoolConfig> {
  const response = await request<Envelope<PoolConfig>>('/api/ops/pool/configs', {
    method: 'POST',
    data: body,
  });
  return response.data;
}

export async function publishPoolConfig(configId: string): Promise<PoolConfig> {
  const response = await request<Envelope<PoolConfig>>(
    `/api/ops/pool/configs/${configId}/publish`,
    { method: 'POST' },
  );
  return response.data;
}

export async function listSpecialPlans(
  params?: MarketingListParams & { state?: string },
): Promise<Envelope<SpecialPricePlan[]>> {
  return request<Envelope<SpecialPricePlan[]>>('/api/ops/pool/plans', { params });
}

export async function createSpecialPlan(body: {
  name: string;
  city_code: string;
  car_type: string;
  fixed_fen: number;
  seats: number;
  valid_from: string;
  valid_to: string;
}): Promise<SpecialPricePlan> {
  const response = await request<Envelope<SpecialPricePlan>>('/api/ops/pool/plans', {
    method: 'POST',
    data: body,
  });
  return response.data;
}

export async function publishSpecialPlan(planId: string): Promise<SpecialPricePlan> {
  const response = await request<Envelope<SpecialPricePlan>>(
    `/api/ops/pool/plans/${planId}/publish`,
    { method: 'POST' },
  );
  return response.data;
}

export async function readPoolStats(): Promise<PoolStats> {
  const response = await request<Envelope<PoolStats>>('/api/ops/pool/stats');
  return response.data;
}

// 导出走原生 fetch：拿到的是 CSV 二进制流，不是 JSON 信封。
export async function exportCampaignSettlement(code: string): Promise<void> {
  const response = await fetch(
    `/api/ops/marketing/campaigns/${code}/settlement/export`,
    { credentials: 'include' },
  );
  if (!response.ok) {
    throw new Error('导出失败');
  }
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `campaign-settlement-${code}.csv`;
  link.click();
  URL.revokeObjectURL(url);
}
