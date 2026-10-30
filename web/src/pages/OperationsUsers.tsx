import {
  ModalForm,
  type ActionType,
  PageContainer,
  type ProColumns,
  ProDescriptions,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Button, Drawer, message, Popconfirm, Space, Tag } from 'antd';
import React, { useRef, useState } from 'react';

import {
  listOpsUsers,
  type OpsRestriction,
  type OpsUserDetail,
  type OpsUserItem,
  readOpsUser,
  restrictOpsUser,
  unrestrictOpsUser,
} from '@/services/operations';

const restrictionTypeLabels: Record<string, string> = {
  NO_NEW_RIDE: '禁新乘车',
  NO_NEW_ORDER: '禁新接单',
  NO_WITHDRAW: '禁提现',
  LOGIN_RISK: '登录风险限制',
};

const roleLabels: Record<string, { text: string; color: string }> = {
  pax: { text: '乘客', color: 'blue' },
  drv: { text: '司机', color: 'orange' },
};

const formatDate = (value?: string) =>
  value ? new Date(value).toLocaleString() : '-';

const restrictionTag = (type: string) => (
  <Tag key={type} color="red">
    {restrictionTypeLabels[type] ?? type}
  </Tag>
);

type RestrictFormValues = {
  type: string;
  reason: string;
  scope: string;
};

const OperationsUsers: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [detail, setDetail] = useState<OpsUserDetail>();
  const [restrictTarget, setRestrictTarget] = useState<OpsUserItem>();
  const [revokeTarget, setRevokeTarget] = useState<{
    user: OpsUserDetail;
    restriction: OpsRestriction;
  }>();

  const openDetail = async (id: string) => {
    setDetail(await readOpsUser(id));
  };

  const reload = () => actionRef.current?.reload();

  const columns: ProColumns<OpsUserItem>[] = [
    {
      title: '关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '手机号后四位或姓名' },
    },
    {
      title: '角色筛选',
      dataIndex: 'role',
      hideInTable: true,
      valueType: 'select',
      fieldProps: {
        placeholder: '按角色筛选',
        allowClear: true,
        options: [
          { label: '乘客', value: 'pax' },
          { label: '司机', value: 'drv' },
        ],
      },
    },
    { title: '手机号（掩码）', dataIndex: 'phone_mask', width: 140 },
    {
      title: '姓名',
      dataIndex: 'real_name',
      width: 120,
      search: false,
    },
    {
      title: '角色',
      dataIndex: 'role_show',
      search: false,
      width: 90,
      render: (_, record) => {
        const role = roleLabels[record.role] ?? {
          text: record.role,
          color: 'default',
        };
        return <Tag color={role.color}>{role.text}</Tag>;
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      search: false,
      width: 90,
      render: (_, record) => (
        <Tag color={record.status === 1 ? 'green' : 'default'}>
          {record.status === 1 ? '正常' : `状态${record.status}`}
        </Tag>
      ),
    },
    {
      title: '生效限制',
      dataIndex: 'restrict',
      search: false,
      render: (_, record) =>
        (record.restrict ?? []).length > 0 ? (
          <Space size={4} wrap>
            {(record.restrict ?? []).map((item) => restrictionTag(item.type))}
          </Space>
        ) : (
          '-'
        ),
    },
    {
      title: '注册时间',
      dataIndex: 'created_at',
      search: false,
      width: 170,
      render: (_, record) => formatDate(record.created_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 140,
      fixed: 'right',
      render: (_, record) => [
        <a key="detail" onClick={() => void openDetail(record.id)}>
          详情
        </a>,
        ...(access.canOpsUserRestrict
          ? [
              <a key="restrict" onClick={() => setRestrictTarget(record)}>
                限制
              </a>,
            ]
          : []),
      ],
    },
  ];

  return (
    <PageContainer title="用户管理">
      <ProTable<OpsUserItem>
        rowKey="id"
        actionRef={actionRef as never}
        columns={columns}
        scroll={{ x: 1000 }}
        request={async (params) => {
          const response = await listOpsUsers({
            page: params.current ?? 1,
            page_size: params.pageSize ?? 20,
            keyword: (params as { keyword?: string }).keyword || undefined,
            role: (params as { role?: string }).role || undefined,
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
        title="用户详情"
        width={560}
        open={Boolean(detail)}
        onClose={() => setDetail(undefined)}
        extra={
          detail && access.canOpsUserRestrict ? (
            <Button type="primary" onClick={() => setRestrictTarget(detail)}>
              新增限制
            </Button>
          ) : undefined
        }
      >
        {detail && (
          <ProDescriptions
            column={1}
            size="small"
            dataSource={detail as unknown as Record<string, unknown>}
            columns={[
              { title: 'ID', dataIndex: 'id' },
              { title: '完整手机号', dataIndex: 'phone' },
              { title: '姓名', dataIndex: 'real_name' },
              {
                title: '角色',
                dataIndex: 'role',
                render: (_, entity) =>
                  (entity as unknown as OpsUserDetail).role === 'drv'
                    ? '司机'
                    : '乘客',
              },
              {
                title: '司机ID',
                dataIndex: 'driver_id',
                render: (_, entity) =>
                  (entity as unknown as OpsUserDetail).driver_id || '-',
              },
              {
                title: '注册时间',
                dataIndex: 'created_at',
                render: (_, entity) =>
                  formatDate((entity as unknown as OpsUserDetail).created_at),
              },
            ]}
          />
        )}
        {detail && (detail.restrict ?? []).length > 0 && (
          <>
            <h4 style={{ margin: '16px 0 8px' }}>生效中的限制</h4>
            {(detail.restrict ?? []).map((restriction) => (
              <RestrictionCard
                key={restriction.id}
                restriction={restriction}
                canRevoke={access.canOpsUserRestrict}
                onRevoke={() =>
                  setRevokeTarget({ user: detail, restriction })
                }
              />
            ))}
          </>
        )}
      </Drawer>
      <ModalForm<RestrictFormValues>
        key={restrictTarget?.id ?? 'idle'}
        title={
          restrictTarget ? `限制用户 ${restrictTarget.phone_mask}` : '限制用户'
        }
        open={Boolean(restrictTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setRestrictTarget(undefined);
          }
        }}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          if (!restrictTarget) {
            return true;
          }
          await restrictOpsUser(restrictTarget.id, values);
          message.success('限制已生效');
          const affected = restrictTarget;
          setRestrictTarget(undefined);
          if (detail?.id === affected.id) {
            void openDetail(affected.id);
          }
          reload();
          return true;
        }}
      >
        <ProFormSelect
          name="type"
          label="限制类型"
          rules={[{ required: true, message: '请选择限制类型' }]}
          options={Object.entries(restrictionTypeLabels).map(
            ([value, label]) => ({ value, label }),
          )}
        />
        <ProFormTextArea
          name="reason"
          label="限制原因"
          rules={[
            { required: true, message: '请填写限制原因' },
            { min: 5, max: 200, message: '原因长度需在 5-200 字之间' },
          ]}
          fieldProps={{ rows: 3, showCount: true, maxLength: 200 }}
        />
        <ProFormText
          name="scope"
          label="限制范围"
          placeholder="如：全平台 / 仅城市A"
          rules={[{ required: true, message: '请填写限制范围' }]}
        />
      </ModalForm>

      <ModalForm<{ reason: string }>
        key={revokeTarget?.restriction.id ?? 'revoke-idle'}
        title="解除限制"
        open={Boolean(revokeTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setRevokeTarget(undefined);
          }
        }}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          if (!revokeTarget) {
            return true;
          }
          const affected = revokeTarget;
          await unrestrictOpsUser(affected.user.id, {
            restriction_id: affected.restriction.id,
            reason: values.reason,
          });
          message.success('限制已解除');
          setRevokeTarget(undefined);
          void openDetail(affected.user.id);
          reload();
          return true;
        }}
      >
        <ProFormTextArea
          name="reason"
          label="解除原因"
          rules={[
            { required: true, message: '请填写解除原因' },
            { min: 5, max: 200, message: '原因长度需在 5-200 字之间' },
          ]}
          fieldProps={{ rows: 3, showCount: true, maxLength: 200 }}
        />
      </ModalForm>
    </PageContainer>
  );
};

type RestrictionCardProps = {
  restriction: OpsRestriction;
  canRevoke: boolean;
  onRevoke: () => void;
};

const RestrictionCard: React.FC<RestrictionCardProps> = ({
  restriction,
  canRevoke,
  onRevoke,
}) => (
  <div
    style={{
      border: '1px solid #f0f0f0',
      borderRadius: 8,
      padding: 12,
      marginBottom: 8,
    }}
  >
    <Space wrap>
      {restrictionTag(restriction.type)}
      <span style={{ color: '#999', fontSize: 12 }}>
        {restriction.operator} · {formatDate(restriction.created_at)}
      </span>
      {canRevoke && (
        <Popconfirm
          title="解除限制"
          description="确认解除该限制？需填写原因留痕。"
          okText="去填写原因"
          onConfirm={onRevoke}
        >
          <a>解除</a>
        </Popconfirm>
      )}
    </Space>
    <div style={{ marginTop: 4 }}>原因：{restriction.reason}</div>
    <div style={{ color: '#999', fontSize: 12 }}>范围：{restriction.scope}</div>
  </div>
);

export default OperationsUsers;
