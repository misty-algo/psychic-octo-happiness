import {
  ModalForm,
  type ActionType,
  PageContainer,
  type ProColumns,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Button, Drawer, message, Popconfirm, Space, Table, Tag } from 'antd';
import React, { useRef, useState } from 'react';

import {
  claimDriverAudit,
  decideDriverAudit,
  driverAuditHistory,
  type AuditHistory,
  type AuditItem,
  type AuditVehicle,
  listDriverAudits,
} from '@/services/operations';

const stateColors: Record<string, string> = {
  待审: 'orange',
  通过: 'green',
  驳回: 'red',
};

const formatDate = (value?: string) =>
  value ? new Date(value).toLocaleString() : '-';

const vehicleText = (vehicle: AuditVehicle) =>
  `${vehicle.plate} · ${vehicle.model} · ${vehicle.seats}座 · 证${vehicle.cert_no}（有效期至 ${vehicle.cert_expire}）`;

const stateFilterOptions = [
  { label: '待审', value: '待审' },
  { label: '通过', value: '通过' },
  { label: '驳回', value: '驳回' },
];

const OperationsDriverAudits: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [history, setHistory] = useState<AuditHistory>();
  const [rejectTarget, setRejectTarget] = useState<AuditItem>();
  const [claiming, setClaiming] = useState<string>();

  const reload = () => actionRef.current?.reload();

  const openHistory = async (applicationId: string) => {
    setHistory(await driverAuditHistory(applicationId));
  };

  const handleClaim = async (record: AuditItem) => {
    setClaiming(record.application_id);
    try {
      await claimDriverAudit(record.application_id, record.revision);
      message.success('领取成功，可以进行裁决');
      reload();
    } finally {
      setClaiming(undefined);
    }
  };

  const handleApprove = async (record: AuditItem) => {
    await decideDriverAudit(record.application_id, record.revision, {
      action: 'approve',
      version: record.version,
    });
    message.success('已通过，资质联动事件已发出');
    reload();
  };

  const columns: ProColumns<AuditItem>[] = [
    {
      title: '关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '申请编号或车牌' },
    },
    {
      title: '状态筛选',
      dataIndex: 'state',
      hideInTable: true,
      valueType: 'select',
      fieldProps: { placeholder: '按状态筛选', allowClear: true, options: stateFilterOptions },
    },
    { title: '申请编号', dataIndex: 'application_id', width: 180, search: false },
    { title: '版本', dataIndex: 'revision', width: 70, search: false },
    {
      title: '司机',
      dataIndex: 'driver_name',
      width: 100,
      search: false,
    },
    { title: '车牌', dataIndex: 'plate', width: 120, search: false },
    {
      title: '车辆资料',
      dataIndex: ['vehicle', 'model'],
      search: false,
      render: (_, record) => vehicleText(record.vehicle),
    },
    {
      title: '状态',
      dataIndex: 'state_show',
      width: 110,
      search: false,
      render: (_, record) => (
        <Space size={4} direction="vertical">
          <Tag color={stateColors[record.state] ?? 'default'}>{record.state}</Tag>
          {record.manual_review && <Tag color="purple">重点复核</Tag>}
        </Space>
      ),
    },
    {
      title: '累计驳回',
      dataIndex: 'rejections',
      width: 90,
      search: false,
    },
    {
      title: '持有人',
      dataIndex: 'claimed_by',
      width: 110,
      search: false,
      render: (_, record) => record.claimed_by || '-',
    },
    {
      title: '驳回原因',
      dataIndex: 'reason',
      search: false,
      ellipsis: true,
      render: (_, record) => record.reason || '-',
    },
    {
      title: '提交时间',
      dataIndex: 'submitted_at',
      width: 170,
      search: false,
      render: (_, record) => formatDate(record.submitted_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 200,
      fixed: 'right',
      render: (_, record) => {
        const actions: React.ReactNode[] = [
          <a
            key="history"
            onClick={() => void openHistory(record.application_id)}
          >
            版本链
          </a>,
        ];
        if (access.canOpsAuditClaim && record.state === '待审') {
          if (!record.claimed_by) {
            actions.push(
              <a
                key="claim"
                onClick={() => void handleClaim(record)}
              >
                {claiming === record.application_id ? '领取中…' : '领取'}
              </a>,
            );
          }
        }
        if (access.canOpsAuditDecide && record.state === '待审') {
          actions.push(
            <Popconfirm
              key="approve"
              title="确认通过？"
              description="通过后将更新司机车辆资料并发出资质变化事件。"
              okText="通过"
              onConfirm={() => void handleApprove(record)}
            >
              <a>通过</a>
            </Popconfirm>,
            <a key="reject" onClick={() => setRejectTarget(record)}>
              驳回
            </a>,
          );
        }
        return actions;
      },
    },
  ];

  return (
    <PageContainer title="司机车辆审核">
      <ProTable<AuditItem>
        rowKey={(record) => `${record.application_id}:${record.revision}`}
        actionRef={actionRef as never}
        columns={columns}
        scroll={{ x: 1200 }}
        request={async (params) => {
          const response = await listDriverAudits({
            page: params.current ?? 1,
            page_size: params.pageSize ?? 20,
            state: (params as { state?: string }).state || undefined,
            keyword: (params as { keyword?: string }).keyword || undefined,
          });
          return {
            data: response.data,
            success: true,
            total: response.page?.total ?? 0,
          };
        }}
        pagination={{ defaultPageSize: 20 }}
      />
      <Drawer
        title={`版本链 ${history?.application_id ?? ''}`}
        width={860}
        open={Boolean(history)}
        onClose={() => setHistory(undefined)}
        extra={
          history && (
            <Space>
              <Tag color="red">累计驳回 {history.rejections}</Tag>
              {history.manual_review && <Tag color="purple">重点复核</Tag>}
            </Space>
          )
        }
      >
        {history && (
          <Table<AuditItem>
            rowKey={(record) => `${record.application_id}:${record.revision}`}
            pagination={false}
            size="small"
            columns={[
              { title: '版本', dataIndex: 'revision', width: 60 },
              {
                title: '状态',
                dataIndex: 'state',
                width: 90,
                render: (_, record) => (
                  <Tag color={stateColors[record.state] ?? 'default'}>
                    {record.state}
                  </Tag>
                ),
              },
              {
                title: '车辆资料',
                render: (_, record) => vehicleText(record.vehicle),
              },
              {
                title: '驳回原因',
                dataIndex: 'reason',
                render: (_, record) => record.reason || '-',
              },
              {
                title: '裁决人',
                dataIndex: 'decided_by',
                width: 110,
                render: (_, record) => record.decided_by || '-',
              },
              {
                title: '提交时间',
                dataIndex: 'submitted_at',
                width: 160,
                render: (_, record) => formatDate(record.submitted_at),
              },
            ]}
            dataSource={history.revisions}
          />
        )}
      </Drawer>
      <ModalForm<{ reason: string }>
        key={rejectTarget?.application_id ?? 'reject-idle'}
        title={
          rejectTarget
            ? `驳回 ${rejectTarget.application_id}（版本 ${rejectTarget.revision}）`
            : '驳回'
        }
        open={Boolean(rejectTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setRejectTarget(undefined);
          }
        }}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          if (!rejectTarget) {
            return true;
          }
          const affected = rejectTarget;
          await decideDriverAudit(
            affected.application_id,
            affected.revision,
            { action: 'reject', reason: values.reason, version: affected.version },
          );
          message.success('已驳回');
          setRejectTarget(undefined);
          reload();
          return true;
        }}
      >
        <ProFormTextArea
          name="reason"
          label="驳回原因（司机端可见）"
          rules={[
            { required: true, message: '驳回必须填写原因' },
            { min: 5, max: 200, message: '原因长度需在 5-200 字之间' },
          ]}
          fieldProps={{ rows: 4, showCount: true, maxLength: 200 }}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default OperationsDriverAudits;
