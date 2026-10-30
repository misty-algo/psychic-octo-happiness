import { useAccess } from '@umijs/max';
import { Modal, message, Tag } from 'antd';
import React, { useRef, useState } from 'react';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';

import {
  approveWithdrawal,
  listWithdrawals,
  payWithdrawal,
  rejectWithdrawal,
  type WithdrawalRow,
} from '@/services/operations';

const yuan = (fen: number | undefined) =>
  ((fen ?? 0) / 100).toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });

const stateColors: Record<string, string> = {
  reviewing: 'orange',
  approved: 'blue',
  paying: 'geekblue',
  success: 'green',
  rejected: 'default',
};

const stateLabels: Record<string, string> = {
  reviewing: '待审核',
  approved: '已核准待打款',
  paying: '打款中',
  success: '已打款',
  rejected: '已驳回',
};

const stateLabel = (state?: string) =>
  (state && stateLabels[state]) || state || '-';

const OperationsWithdrawals: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [rejectTarget, setRejectTarget] = useState<WithdrawalRow | null>(null);
  const [rejectReason, setRejectReason] = useState('');

  const reload = () => actionRef.current?.reload();

  const onApprove = (row: WithdrawalRow) => {
    Modal.confirm({
      title: `通过提现 ¥${yuan(row.amount_fen)}？`,
      content: row.dual_approval
        ? '该笔为大额提现需双人审批：您通过后若审批未凑齐，需另一位审核员再通过一次才会打款。'
        : '通过后将立即打款，金额从司机「提现中」桶出账，此操作不可撤销。',
      okText: '通过并打款',
      cancelText: '取消',
      onOk: async () => {
        try {
          const result = await approveWithdrawal(row.id, {});
          message.success(result.message || `状态：${result.status}`);
          reload();
        } catch (error) {
          message.error((error as Error).message || '审批失败');
        }
      },
    });
  };

  const onReject = () => {
    if (!rejectTarget) return;
    const reason = rejectReason.trim();
    if (reason.length < 5) {
      message.warning('请填写驳回原因（至少 5 字）');
      return;
    }
    Modal.confirm({
      title: `驳回提现 ¥${yuan(rejectTarget.amount_fen)}？`,
      content: '驳回后预占金额将返还司机可用余额。',
      okText: '确认驳回',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        try {
          const result = await rejectWithdrawal(rejectTarget.id, { reason });
          message.success(result.message || `状态：${result.status}`);
          setRejectTarget(null);
          setRejectReason('');
          reload();
        } catch (error) {
          message.error((error as Error).message || '驳回失败');
        }
      },
    });
  };

  const onPay = (row: WithdrawalRow) => {
    Modal.confirm({
      title: `对已核准单补发打款 ¥${yuan(row.amount_fen)}？`,
      okText: '确认打款',
      onOk: async () => {
        try {
          const result = await payWithdrawal(row.id);
          message.success(result.message || `状态：${result.status}`);
          reload();
        } catch (error) {
          message.error((error as Error).message || '打款失败');
        }
      },
    });
  };

  const columns: ProColumns<WithdrawalRow>[] = [
    {
      title: '提现单号',
      dataIndex: 'id',
      width: 240,
      ellipsis: true,
      copyable: true,
      search: false,
    },
    {
      title: '司机',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '提现单号 / 司机ID' },
    },
    {
      title: '司机',
      dataIndex: 'driver_name',
      width: 150,
      search: false,
      render: (_, record) =>
        `${record.driver_name || '-'}${
          record.driver_no ? ` · ${record.driver_no}` : ''
        }`,
    },
    {
      title: '金额',
      dataIndex: 'amount_fen',
      width: 110,
      search: false,
      render: (_, record) => `¥${yuan(record.amount_fen)}`,
    },
    {
      title: '状态',
      dataIndex: 'state',
      width: 120,
      valueType: 'select',
      fieldProps: {
        options: Object.keys(stateColors).map((value) => ({
          label: stateLabels[value] ?? value,
          value,
        })),
      },
      render: (_, record) => (
        <Tag color={stateColors[record.status] ?? 'default'}>
          {stateLabel(record.status)}
        </Tag>
      ),
    },
    {
      title: '审批',
      dataIndex: 'approvals',
      width: 110,
      search: false,
      render: (_, record) =>
        record.dual_approval
          ? `双人审批 ${record.approvals}/2`
          : `普通 ${record.approvals}/1`,
    },
    {
      title: '申请时间',
      dataIndex: 'created_at',
      width: 170,
      search: false,
      render: (_, record) =>
        record.created_at
          ? new Date(record.created_at).toLocaleString('zh-CN', { hour12: false })
          : '-',
    },
    {
      title: '操作',
      valueType: 'option',
      width: 200,
      fixed: 'right',
      render: (_, record) => {
        if (!access.canOpsOrderCommand) return [];
        const actions: React.ReactNode[] = [];
        if (record.status === 'reviewing') {
          actions.push(
            <a key="approve" onClick={() => onApprove(record)}>
              通过
            </a>,
            <a key="reject" style={{ color: '#f56c6c' }} onClick={() => setRejectTarget(record)}>
              驳回
            </a>,
          );
        }
        if (record.status === 'approved' || record.status === 'paying') {
          actions.push(
            <a key="pay" onClick={() => onPay(record)}>
              打款
            </a>,
          );
        }
        return actions;
      },
    },
  ];

  return (
    <PageContainer title="提现审批">
      <ProTable<WithdrawalRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        scroll={{ x: 1100 }}
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const query = params as {
            current?: number;
            pageSize?: number;
            state?: string;
            keyword?: string;
          };
          const response = await listWithdrawals({
            page: query.current ?? 1,
            page_size: query.pageSize ?? 20,
            state: query.state || undefined,
            keyword: query.keyword || undefined,
          });
          return {
            data: response.data ?? [],
            total: response.page?.total ?? response.data?.length ?? 0,
            success: true,
          };
        }}
        pagination={{ defaultPageSize: 20 }}
      />
      <Modal
        title={`驳回提现 ¥${yuan(rejectTarget?.amount_fen)}`}
        open={!!rejectTarget}
        onOk={onReject}
        onCancel={() => {
          setRejectTarget(null);
          setRejectReason('');
        }}
        okText="确认驳回"
        okButtonProps={{ danger: true }}
        destroyOnClose
      >
        <div style={{ marginBottom: 8, color: '#909399' }}>
          司机：{rejectTarget?.driver_name || '-'} · 驳回后预占金额返还司机可用余额
        </div>
        <textarea
          style={{ width: '100%', minHeight: 80, padding: 8 }}
          placeholder="请填写驳回原因（至少 5 字）"
          value={rejectReason}
          onChange={(event) => setRejectReason(event.target.value)}
        />
      </Modal>
    </PageContainer>
  );
};

export default OperationsWithdrawals;
