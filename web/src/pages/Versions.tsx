import {
  DeleteOutlined,
  ExportOutlined,
  ImportOutlined,
  PlusOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  type ProColumns,
  ProDescriptions,
  ProFormDateTimePicker,
  type ProFormInstance,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Button, Drawer, message, Popconfirm, Upload } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import React, { useRef, useState } from 'react';

import {
  batchDeleteVersions,
  createVersion,
  type Dictionary,
  deleteVersion,
  downloadVersionJSON,
  type ExportVersionInput,
  exportVersion,
  importVersion,
  listDictionaries,
  listMenus,
  listVersions,
  type Menu,
  readVersion,
  type SystemVersion,
  updateVersion,
  type VersionBundle,
} from '@/services/admin';

type VersionFormValues = {
  version: string;
  name: string;
  description?: string;
  published_at?: Dayjs;
};

type ExportFormValues = {
  version: string;
  name: string;
  description?: string;
  menu_ids?: number[];
  dictionary_ids?: number[];
};

type ExportResources = {
  menus: Menu[];
  dictionaries: Dictionary[];
};

const emptyResources: ExportResources = {
  menus: [],
  dictionaries: [],
};

const formatDate = (value?: string) =>
  value ? new Date(value).toLocaleString() : '-';

const versionRules = [
  { required: true, message: '请输入版本号' },
  {
    pattern: /^[A-Za-z0-9._+-]+$/,
    message: '版本号只能包含字母、数字、点、横线、下划线和加号',
  },
];

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

// 导入包只接受菜单和字典定义，包含 API 路由定义的包会被拒绝。
const isVersionBundle = (value: unknown): value is VersionBundle => {
  if (!isRecord(value) || !isRecord(value.version)) {
    return false;
  }
  return (
    typeof value.version.code === 'string' &&
    typeof value.version.name === 'string' &&
    (value.menus === undefined || Array.isArray(value.menus)) &&
    value.apis === undefined &&
    (value.dictionaries === undefined || Array.isArray(value.dictionaries))
  );
};

const saveBlob = (blob: Blob, filename: string) => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
};

const Versions: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const importFormRef = useRef<ProFormInstance>(undefined);
  const [selectedVersionIDs, setSelectedVersionIDs] = useState<React.Key[]>([]);
  const [resources, setResources] = useState<ExportResources>(emptyResources);
  const [modalOpen, setModalOpen] = useState(false);
  const [exportModalOpen, setExportModalOpen] = useState(false);
  const [importModalOpen, setImportModalOpen] = useState(false);
  const [editing, setEditing] = useState<SystemVersion>();
  const [detail, setDetail] = useState<SystemVersion>();

  const loadResources = async () => {
    const [menus, dictionaries] = await Promise.all([
      access.canMenuRead ? listMenus() : Promise.resolve([]),
      access.canDictRead ? listDictionaries() : Promise.resolve([]),
    ]);
    setResources({ menus, dictionaries });
  };

  const downloadVersion = async (record: SystemVersion) => {
    try {
      const blob = await downloadVersionJSON(record.id);
      saveBlob(blob, `version_${record.version}.json`);
      message.success('版本JSON已下载');
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : '版本JSON下载失败',
      );
    }
  };

  const columns: ProColumns<SystemVersion>[] = [
    { title: '版本号', dataIndex: 'version', width: 160 },
    { title: '名称', dataIndex: 'name', width: 200 },
    { title: '说明', dataIndex: 'description', ellipsis: true },
    {
      title: '发布时间',
      dataIndex: 'published_at',
      width: 200,
      render: (_, record) => formatDate(record.published_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 220,
      render: (_, record) => {
        const actions: React.ReactNode[] = [
          <a
            key="detail"
            onClick={() => {
              void readVersion(record.id).then(setDetail);
            }}
          >
            详情
          </a>,
        ];
        if (access.canVersionUpdate) {
          actions.push(
            <a
              key="edit"
              onClick={() => {
                setEditing(record);
                setModalOpen(true);
              }}
            >
              编辑
            </a>,
          );
        }
        if (access.canVersionRead) {
          actions.push(
            <a key="download" onClick={() => void downloadVersion(record)}>
              下载
            </a>,
          );
        }
        if (access.canVersionDelete) {
          actions.push(
            <Popconfirm
              key="delete"
              title="删除版本记录"
              description={`确认删除 ${record.version}？`}
              okText="删除"
              okButtonProps={{ danger: true }}
              onConfirm={async () => {
                await deleteVersion(record.id);
                message.success('版本记录已删除');
                setSelectedVersionIDs((previous) =>
                  previous.filter((id) => id !== record.id),
                );
                actionRef.current?.reload();
              }}
            >
              <Button type="link" danger size="small">
                删除
              </Button>
            </Popconfirm>,
          );
        }
        return actions;
      },
    },
  ];

  const menuOptions = resources.menus.map((menu) => ({
    label: `${menu.name} (${menu.path})`,
    value: menu.id,
  }));
  const dictionaryOptions = resources.dictionaries.map((dictionary) => ({
    label: `${dictionary.name} (${dictionary.code})`,
    value: dictionary.id,
  }));

  return (
    <PageContainer title="版本管理">
      <ProTable<SystemVersion>
        headerTitle="版本列表"
        rowKey="id"
        actionRef={actionRef}
        search={false}
        pagination={false}
        columns={columns}
        rowSelection={
          access.canVersionDelete
            ? {
                selectedRowKeys: selectedVersionIDs,
                onChange: (keys) => setSelectedVersionIDs(keys),
              }
            : false
        }
        request={async () => {
          const data = await listVersions();
          return { data, success: true, total: data.length };
        }}
        toolBarRender={() => {
          const buttons: React.ReactNode[] = [];
          if (access.canVersionCreate) {
            buttons.push(
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setEditing(undefined);
                  setModalOpen(true);
                }}
              >
                新增版本
              </Button>,
              <Button
                key="export"
                icon={<ExportOutlined />}
                onClick={() => {
                  setExportModalOpen(true);
                  void loadResources();
                }}
              >
                导出
              </Button>,
              <Button
                key="import"
                icon={<ImportOutlined />}
                onClick={() => setImportModalOpen(true)}
              >
                导入
              </Button>,
            );
          }
          if (access.canVersionDelete && selectedVersionIDs.length > 0) {
            buttons.push(
              <Popconfirm
                key="batch-delete"
                title="批量删除版本记录"
                description={`确认删除选中的 ${selectedVersionIDs.length} 条记录？`}
                okText="删除"
                okButtonProps={{ danger: true }}
                onConfirm={async () => {
                  await batchDeleteVersions(selectedVersionIDs.map(Number));
                  message.success('版本记录已批量删除');
                  setSelectedVersionIDs([]);
                  actionRef.current?.reload();
                }}
              >
                <Button danger icon={<DeleteOutlined />}>
                  批量删除
                </Button>
              </Popconfirm>,
            );
          }
          return buttons;
        }}
      />
      <ModalForm<VersionFormValues>
        key={editing?.id ?? 'create'}
        title={editing ? '编辑版本' : '新增版本'}
        open={modalOpen}
        onOpenChange={setModalOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={
          editing
            ? {
                version: editing.version,
                name: editing.name,
                description: editing.description,
                published_at: dayjs(editing.published_at),
              }
            : { published_at: dayjs() }
        }
        onFinish={async (values) => {
          const input = {
            version: values.version,
            name: values.name,
            description: values.description,
            published_at: values.published_at?.toISOString(),
          };
          if (editing) {
            await updateVersion(editing.id, input);
            message.success('版本记录已更新');
          } else {
            await createVersion(input);
            message.success('版本记录已创建');
          }
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText
          name="version"
          label="版本号"
          fieldProps={{ maxLength: 80 }}
          rules={versionRules}
        />
        <ProFormText
          name="name"
          label="名称"
          fieldProps={{ maxLength: 120 }}
          rules={[{ required: true, message: '请输入版本名称' }]}
        />
        <ProFormTextArea
          name="description"
          label="说明"
          fieldProps={{ maxLength: 4000, rows: 5 }}
        />
        <ProFormDateTimePicker
          name="published_at"
          label="发布时间"
          fieldProps={{ showTime: true, style: { width: '100%' } }}
          rules={[{ required: true, message: '请选择发布时间' }]}
        />
      </ModalForm>
      <ModalForm<ExportFormValues>
        key="export"
        title="导出版本包"
        open={exportModalOpen}
        onOpenChange={setExportModalOpen}
        width={520}
        modalProps={{ destroyOnHidden: true }}
        initialValues={{
          version: `v${dayjs().format('YYYY.MM.DD.HHmm')}`,
          name: '后台配置包',
        }}
        onFinish={async (values) => {
          const input: ExportVersionInput = {
            version: values.version,
            name: values.name,
            description: values.description,
            menu_ids: values.menu_ids ?? [],
            dictionary_ids: values.dictionary_ids ?? [],
          };
          const created = await exportVersion(input);
          message.success('版本包已导出');
          actionRef.current?.reload();
          await downloadVersion(created);
          return true;
        }}
      >
        <ProFormText
          name="version"
          label="版本号"
          fieldProps={{ maxLength: 80 }}
          rules={versionRules}
        />
        <ProFormText
          name="name"
          label="名称"
          fieldProps={{ maxLength: 120 }}
          rules={[{ required: true, message: '请输入版本名称' }]}
        />
        <ProFormTextArea
          name="description"
          label="说明"
          fieldProps={{ maxLength: 4000, rows: 3 }}
        />
        <ProFormSelect
          name="menu_ids"
          label="菜单"
          mode="multiple"
          options={menuOptions}
          fieldProps={{ allowClear: true, maxTagCount: 'responsive' }}
        />
        <ProFormSelect
          name="dictionary_ids"
          label="字典"
          mode="multiple"
          options={dictionaryOptions}
          fieldProps={{ allowClear: true, maxTagCount: 'responsive' }}
        />
      </ModalForm>
      <ModalForm<{ data: string }>
        key="import"
        title="导入版本包"
        open={importModalOpen}
        onOpenChange={setImportModalOpen}
        width={640}
        formRef={importFormRef}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          let parsed: unknown;
          try {
            parsed = JSON.parse(values.data);
          } catch {
            message.error('JSON格式不正确');
            return false;
          }
          if (!isVersionBundle(parsed)) {
            message.error('版本包结构不正确');
            return false;
          }
          await importVersion(parsed);
          message.success('版本包已导入');
          actionRef.current?.reload();
          return true;
        }}
      >
        <Upload
          accept="application/json,.json"
          maxCount={1}
          showUploadList={false}
          beforeUpload={(file) => {
            const reader = new FileReader();
            reader.onload = () => {
              importFormRef.current?.setFieldsValue({
                data: typeof reader.result === 'string' ? reader.result : '',
              });
            };
            reader.onerror = () => {
              message.error('文件读取失败');
            };
            reader.readAsText(file);
            return false;
          }}
        >
          <Button icon={<UploadOutlined />}>选择JSON文件</Button>
        </Upload>
        <ProFormTextArea
          name="data"
          label="JSON内容"
          fieldProps={{ rows: 12 }}
          rules={[{ required: true, message: '请粘贴或选择版本JSON' }]}
        />
      </ModalForm>
      <Drawer
        title="版本详情"
        width={480}
        open={Boolean(detail)}
        onClose={() => setDetail(undefined)}
      >
        {detail && (
          <ProDescriptions<SystemVersion>
            column={1}
            size="small"
            dataSource={detail}
            columns={[
              { title: '版本号', dataIndex: 'version' },
              { title: '名称', dataIndex: 'name' },
              {
                title: '说明',
                dataIndex: 'description',
                render: (_, entity) => entity.description || '-',
              },
              {
                title: '发布时间',
                dataIndex: 'published_at',
                render: (_, entity) => formatDate(entity.published_at),
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

export default Versions;
