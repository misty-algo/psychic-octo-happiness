import {
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  PlusOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  type ProColumns,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Button, message, Popconfirm, Space, Table, Tag, Upload } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import React, { useRef, useState } from 'react';

import {
  addDictionaryItem,
  createDictionary,
  type Dictionary,
  type DictionaryBundle,
  type DictionaryItem,
  type DictionaryItemInput,
  deleteDictionary,
  deleteDictionaryItem,
  exportDictionaries,
  importDictionaries,
  listDictionaries,
  updateDictionary,
  updateDictionaryItem,
} from '@/services/admin';

type ItemTarget = {
  code: string;
  dictionary: Dictionary;
  item?: DictionaryItem;
  parentID?: number;
};

type ItemOption = {
  id: number;
  label: string;
  path: string;
};

const flattenItems = (items: DictionaryItem[], depth = 0): ItemOption[] =>
  items.flatMap((item) => [
    {
      id: item.id,
      label: `${'  '.repeat(depth)}${item.label}`,
      path: item.path,
    },
    ...flattenItems(item.children, depth + 1),
  ]);

// 编辑字典项时，父级选项要排除自身及其后代，避免形成环。
const parentOptions = (target?: ItemTarget): ItemOption[] => {
  if (!target) {
    return [];
  }
  const editingID = target.item?.id;
  return flattenItems(target.dictionary.items).filter((item) => {
    if (!editingID) {
      return true;
    }
    if (item.id === editingID) {
      return false;
    }
    return !item.path
      .split(',')
      .filter(Boolean)
      .map(Number)
      .includes(editingID);
  });
};

const countItems = (items: DictionaryItem[]): number =>
  items.reduce((total, item) => total + 1 + countItems(item.children), 0);

const Dictionaries: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [dictionaryModalOpen, setDictionaryModalOpen] = useState(false);
  const [editingDictionary, setEditingDictionary] = useState<Dictionary>();
  const [itemTarget, setItemTarget] = useState<ItemTarget>();

  const openItemModal = (
    dictionary: Dictionary,
    item?: DictionaryItem,
    parentID?: number,
  ) => {
    setItemTarget({ code: dictionary.code, dictionary, item, parentID });
  };

  const downloadDictionaries = async () => {
    const blob = await exportDictionaries();
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = 'dictionaries.json';
    anchor.click();
    URL.revokeObjectURL(url);
  };

  const importDictionaryFile = async (file: File) => {
    let bundle: DictionaryBundle;
    try {
      bundle = JSON.parse(await file.text()) as DictionaryBundle;
    } catch {
      message.error('字典文件不是有效JSON');
      return Upload.LIST_IGNORE;
    }
    await importDictionaries(bundle);
    message.success('字典已导入');
    actionRef.current?.reload();
    return Upload.LIST_IGNORE;
  };

  const itemColumns = (dictionary: Dictionary): ColumnsType<DictionaryItem> => [
    { title: '标签', dataIndex: 'label' },
    { title: '值', dataIndex: 'value' },
    {
      title: '扩展',
      dataIndex: 'extend',
      render: (extend?: string) => extend || '-',
    },
    { title: '层级', dataIndex: 'level', width: 80 },
    { title: '排序', dataIndex: 'sort', width: 88 },
    {
      title: '状态',
      dataIndex: 'active',
      render: (active: boolean) => (
        <Tag color={active ? 'green' : 'default'}>
          {active ? '启用' : '停用'}
        </Tag>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      render: (_, item) => {
        const actions: React.ReactNode[] = [];
        if (access.canDictCreate) {
          actions.push(
            <Button
              key="create-child"
              type="link"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => openItemModal(dictionary, undefined, item.id)}
            >
              新增子项
            </Button>,
          );
        }
        if (access.canDictUpdate) {
          actions.push(
            <Button
              key="edit"
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => openItemModal(dictionary, item)}
            >
              编辑
            </Button>,
          );
        }
        if (access.canDictDelete) {
          actions.push(
            <Popconfirm
              key="delete"
              title="删除字典项"
              description={`确认删除 ${item.label}？`}
              okText="删除"
              okButtonProps={{ danger: true }}
              onConfirm={async () => {
                await deleteDictionaryItem(dictionary.code, item.id);
                message.success('字典项已删除');
                actionRef.current?.reload();
              }}
            >
              <Button type="link" danger size="small" icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>,
          );
        }
        return actions.length > 0 ? <Space size="small">{actions}</Space> : '-';
      },
    },
  ];

  const columns: ProColumns<Dictionary>[] = [
    { title: '编码', dataIndex: 'code' },
    { title: '名称', dataIndex: 'name' },
    {
      title: '字典项',
      dataIndex: 'items',
      render: (_, record) => `${countItems(record.items)} 项`,
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_, record) => {
        const actions: React.ReactNode[] = [];
        if (access.canDictCreate) {
          actions.push(
            <a key="create-item" onClick={() => openItemModal(record)}>
              新增字典项
            </a>,
          );
        }
        if (access.canDictUpdate) {
          actions.push(
            <a
              key="edit"
              onClick={() => {
                setEditingDictionary(record);
                setDictionaryModalOpen(true);
              }}
            >
              编辑
            </a>,
          );
        }
        if (access.canDictDelete) {
          actions.push(
            <Popconfirm
              key="delete"
              title="删除字典"
              description={`确认删除 ${record.name}？`}
              okText="删除"
              okButtonProps={{ danger: true }}
              onConfirm={async () => {
                await deleteDictionary(record.code);
                message.success('字典已删除');
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

  return (
    <PageContainer title="数据字典">
      <ProTable<Dictionary>
        headerTitle="字典列表"
        rowKey="code"
        actionRef={actionRef}
        search={false}
        pagination={false}
        columns={columns}
        expandable={{
          expandedRowRender: (record) => (
            <Table<DictionaryItem>
              rowKey="id"
              columns={itemColumns(record)}
              dataSource={record.items}
              pagination={false}
              size="small"
            />
          ),
        }}
        request={async () => {
          const data = await listDictionaries();
          return { data, success: true, total: data.length };
        }}
        toolBarRender={() => {
          const buttons: React.ReactNode[] = [];
          if (access.canDictCreate) {
            buttons.push(
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setEditingDictionary(undefined);
                  setDictionaryModalOpen(true);
                }}
              >
                新增字典
              </Button>,
              <Upload
                key="import"
                accept="application/json"
                showUploadList={false}
                beforeUpload={(file) => importDictionaryFile(file)}
              >
                <Button icon={<UploadOutlined />}>导入</Button>
              </Upload>,
            );
          }
          if (access.canDictRead) {
            buttons.push(
              <Button
                key="export"
                icon={<DownloadOutlined />}
                onClick={() => void downloadDictionaries()}
              >
                导出
              </Button>,
            );
          }
          return buttons;
        }}
      />
      <ModalForm<{ code: string; name: string }>
        key={editingDictionary?.code ?? 'create'}
        title={editingDictionary ? '编辑字典' : '新增字典'}
        open={dictionaryModalOpen}
        onOpenChange={setDictionaryModalOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={editingDictionary ?? { code: '', name: '' }}
        onFinish={async (values) => {
          if (editingDictionary) {
            await updateDictionary(editingDictionary.code, {
              name: values.name,
            });
            message.success('字典已更新');
          } else {
            await createDictionary(values);
            message.success('字典已创建');
          }
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText
          name="code"
          label="编码"
          disabled={Boolean(editingDictionary)}
          fieldProps={{ maxLength: 80 }}
          rules={[{ required: true, message: '请输入字典编码' }]}
        />
        <ProFormText
          name="name"
          label="名称"
          fieldProps={{ maxLength: 120 }}
          rules={[{ required: true, message: '请输入字典名称' }]}
        />
      </ModalForm>
      <ModalForm<DictionaryItemInput>
        key={itemTarget?.item?.id ?? `create-${itemTarget?.parentID ?? 'root'}`}
        title={itemTarget?.item ? '编辑字典项' : '新增字典项'}
        open={Boolean(itemTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setItemTarget(undefined);
          }
        }}
        modalProps={{ destroyOnHidden: true }}
        initialValues={
          itemTarget?.item ?? {
            parent_id: itemTarget?.parentID,
            label: '',
            value: '',
            extend: '',
            sort: 100,
            active: true,
          }
        }
        onFinish={async (values) => {
          if (!itemTarget) {
            return true;
          }
          if (itemTarget.item) {
            await updateDictionaryItem(
              itemTarget.code,
              itemTarget.item.id,
              values,
            );
            message.success('字典项已更新');
          } else {
            await addDictionaryItem(itemTarget.code, values);
            message.success('字典项已创建');
          }
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormSelect
          name="parent_id"
          label="父级"
          options={parentOptions(itemTarget).map((item) => ({
            label: item.label,
            value: item.id,
          }))}
          fieldProps={{ allowClear: true, showSearch: true }}
        />
        <ProFormText
          name="label"
          label="标签"
          fieldProps={{ maxLength: 120 }}
          rules={[{ required: true, message: '请输入标签' }]}
        />
        <ProFormTextArea
          name="extend"
          label="扩展值"
          fieldProps={{ maxLength: 4000, rows: 3 }}
          rules={[{ max: 4000 }]}
        />
        <ProFormText
          name="value"
          label="值"
          fieldProps={{ maxLength: 120 }}
          rules={[{ required: true, message: '请输入值' }]}
        />
        <ProFormDigit name="sort" label="排序" min={0} />
        <ProFormSwitch name="active" label="启用" />
      </ModalForm>
    </PageContainer>
  );
};

export default Dictionaries;
