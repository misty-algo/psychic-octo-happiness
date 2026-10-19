import {
  type ActionType,
  PageContainer,
  type ProColumns,
  ProDescriptions,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Drawer, Tag } from 'antd';
import React, { useRef, useState } from 'react';

import {
  type APIResource,
  listAPIs,
  pageParams,
  readAPI,
  toTableResult,
} from '@/services/admin';

const methodColor: Record<string, string> = {
  GET: 'blue',
  POST: 'green',
  PUT: 'gold',
  PATCH: 'purple',
  DELETE: 'red',
};

const formatDate = (value?: string) =>
  value ? new Date(value).toLocaleString() : '-';

const APIs: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [detail, setDetail] = useState<APIResource>();

  const columns: ProColumns<APIResource>[] = [
    {
      title: '方法',
      dataIndex: 'method',
      width: 88,
      render: (_, record) => (
        <Tag color={methodColor[record.method] ?? 'default'}>
          {record.method}
        </Tag>
      ),
    },
    { title: '注册路由模式', dataIndex: 'path', ellipsis: true },
    { title: '描述', dataIndex: 'description', ellipsis: true },
    { title: '分组', dataIndex: 'group', width: 110 },
    {
      title: '绑定权限',
      dataIndex: 'permission',
      width: 140,
      ellipsis: true,
      render: (_, record) => record.permission || '自助路由',
    },
    {
      title: '操作',
      valueType: 'option',
      width: 80,
      render: (_, record) => [
        <a
          key="detail"
          onClick={() => {
            void readAPI(record.id).then(setDetail);
          }}
        >
          详情
        </a>,
      ],
    },
  ];

  return (
    <PageContainer
      title="受管 API 路由目录"
      subTitle="路由身份和元数据由部署代码维护，后台只读；角色对路由的放行由功能权限自动派生"
    >
      <ProTable<APIResource>
        headerTitle={access.canApiRead ? '路由列表' : undefined}
        rowKey="id"
        actionRef={actionRef}
        search={false}
        columns={columns}
        request={async (params) =>
          toTableResult(await listAPIs(pageParams(params)))
        }
      />
      <Drawer
        title="API详情"
        width={520}
        open={Boolean(detail)}
        onClose={() => setDetail(undefined)}
      >
        {detail && (
          <ProDescriptions<APIResource>
            column={1}
            size="small"
            dataSource={detail}
            columns={[
              {
                title: '方法',
                dataIndex: 'method',
                render: (_, entity) => (
                  <Tag color={methodColor[entity.method] ?? 'default'}>
                    {entity.method}
                  </Tag>
                ),
              },
              { title: '注册路由模式', dataIndex: 'path' },
              { title: '描述', dataIndex: 'description' },
              { title: '分组', dataIndex: 'group' },
              {
                title: '绑定权限',
                dataIndex: 'permission',
                render: (_, entity) =>
                  entity.permission || '自助路由（登录即用）',
              },
              {
                title: '创建时间',
                dataIndex: 'created_at',
                render: (_, entity) => formatDate(entity.created_at),
              },
              {
                title: '更新时间',
                dataIndex: 'updated_at',
                render: (_, entity) => formatDate(entity.updated_at),
              },
            ]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default APIs;
