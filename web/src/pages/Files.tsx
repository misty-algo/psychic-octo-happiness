import {
  DeleteOutlined,
  EditOutlined,
  FolderAddOutlined,
  LinkOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  type ProColumns,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Button, message, Popconfirm, Space, Tree, Upload } from 'antd';
import type { DataNode } from 'antd/es/tree';
import React, { useEffect, useRef, useState } from 'react';

import {
  createFileCategory,
  deleteFile,
  deleteFileCategory,
  type FileCategory,
  type FileObject,
  importFileURL,
  listFileCategories,
  listFiles,
  pageParams,
  renameFile,
  toTableResult,
  updateFileCategory,
  uploadFile,
} from '@/services/admin';

const formatDate = (value: string) => new Date(value).toLocaleString();

type CategoryOption = {
  id: number;
  parent_id: number;
  name: string;
  label: string;
};

const categoryNode = (category: FileCategory): DataNode => ({
  key: String(category.id),
  title: category.name,
  children: category.children.map(categoryNode),
});

const categoryTreeData = (categories: FileCategory[]): DataNode[] => [
  {
    key: '0',
    title: '全部文件',
    children: categories.map(categoryNode),
  },
];

const flattenCategories = (
  categories: FileCategory[],
  depth = 0,
): CategoryOption[] =>
  categories.flatMap((category) => [
    {
      id: category.id,
      parent_id: category.parent_id,
      name: category.name,
      label: `${'  '.repeat(depth)}${category.name}`,
    },
    ...flattenCategories(category.children, depth + 1),
  ]);

const Files: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [categories, setCategories] = useState<FileCategory[]>([]);
  const [categoryLoading, setCategoryLoading] = useState(false);
  const [selectedCategoryID, setSelectedCategoryID] = useState(0);
  const [importOpen, setImportOpen] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);
  const [categoryOpen, setCategoryOpen] = useState(false);
  const [renamingFile, setRenamingFile] = useState<FileObject>();
  const [editingCategory, setEditingCategory] = useState<CategoryOption>();
  const [deletingCategoryID, setDeletingCategoryID] = useState<number>();

  const loadCategories = async () => {
    setCategoryLoading(true);
    try {
      setCategories(await listFileCategories());
    } finally {
      setCategoryLoading(false);
    }
  };

  useEffect(() => {
    void loadCategories();
  }, []);

  const categoryOptions = flattenCategories(categories);
  const selectedCategory = categoryOptions.find(
    (category) => category.id === selectedCategoryID,
  );
  const categoryNameByID = new Map(
    categoryOptions.map((category) => [category.id, category.name]),
  );

  const columns: ProColumns<FileObject>[] = [
    {
      title: '文件名',
      dataIndex: 'name',
      render: (_, record) => (
        <a href={record.url} target="_blank" rel="noreferrer">
          {record.name}
        </a>
      ),
    },
    {
      title: '分类',
      dataIndex: 'category_id',
      width: 140,
      render: (_, record) =>
        record.category_id > 0
          ? (categoryNameByID.get(record.category_id) ??
            `#${record.category_id}`)
          : '未分类',
    },
    { title: '类型', dataIndex: 'content_type' },
    {
      title: '大小',
      dataIndex: 'size',
      render: (_, record) =>
        record.content_type === 'external/url'
          ? '外部链接'
          : `${record.size} B`,
    },
    {
      title: '上传时间',
      dataIndex: 'created_at',
      render: (_, record) => formatDate(record.created_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 150,
      render: (_, record) => {
        const actions: React.ReactNode[] = [];
        if (access.canFileUpdate) {
          actions.push(
            <a
              key="rename"
              onClick={() => {
                setRenamingFile(record);
                setRenameOpen(true);
              }}
            >
              重命名
            </a>,
          );
        }
        if (access.canFileDelete) {
          actions.push(
            <Popconfirm
              key="delete"
              title="删除文件"
              description="确认删除这条文件记录？"
              okText="删除"
              okButtonProps={{ danger: true }}
              cancelText="取消"
              onConfirm={async () => {
                await deleteFile(record.id);
                message.success('文件已删除');
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
    <PageContainer title="文件上传">
      <div
        style={{
          display: 'grid',
          gap: 16,
          gridTemplateColumns: '260px minmax(0, 1fr)',
          alignItems: 'start',
        }}
      >
        <div>
          <Space style={{ marginBottom: 12 }} wrap>
            {access.canFileCategoryCreate ? (
              <Button
                icon={<FolderAddOutlined />}
                onClick={() => {
                  setEditingCategory(undefined);
                  setCategoryOpen(true);
                }}
              >
                新增分类
              </Button>
            ) : null}
            {selectedCategory && access.canFileCategoryUpdate ? (
              <Button
                icon={<EditOutlined />}
                onClick={() => {
                  setEditingCategory(selectedCategory);
                  setCategoryOpen(true);
                }}
              >
                编辑
              </Button>
            ) : null}
            {selectedCategory && access.canFileCategoryDelete ? (
              <Popconfirm
                title="删除分类"
                description="确认删除当前分类？"
                okText="删除"
                okButtonProps={{ danger: true }}
                cancelText="取消"
                onConfirm={async () => {
                  setDeletingCategoryID(selectedCategory.id);
                  try {
                    await deleteFileCategory(selectedCategory.id);
                    message.success('分类已删除');
                    setSelectedCategoryID(0);
                    await loadCategories();
                    actionRef.current?.reload();
                  } finally {
                    setDeletingCategoryID(undefined);
                  }
                }}
              >
                <Button
                  danger
                  icon={<DeleteOutlined />}
                  loading={deletingCategoryID === selectedCategory.id}
                >
                  删除
                </Button>
              </Popconfirm>
            ) : null}
          </Space>
          <Tree
            blockNode
            defaultExpandAll
            disabled={categoryLoading}
            selectedKeys={[String(selectedCategoryID)]}
            treeData={categoryTreeData(categories)}
            onSelect={(keys) => {
              setSelectedCategoryID(Number(keys[0] ?? 0));
            }}
          />
        </div>
        <ProTable<FileObject>
          headerTitle="文件列表"
          rowKey="id"
          actionRef={actionRef}
          search={false}
          columns={columns}
          params={{
            category_id:
              selectedCategoryID > 0 ? selectedCategoryID : undefined,
          }}
          request={async (params) =>
            toTableResult(
              await listFiles({
                ...pageParams(params),
                category_id:
                  typeof params.category_id === 'number' &&
                  params.category_id > 0
                    ? params.category_id
                    : undefined,
              }),
            )
          }
          toolBarRender={() =>
            access.canFileUpload
              ? [
                  <Upload
                    key="upload"
                    maxCount={1}
                    showUploadList={false}
                    beforeUpload={async (file) => {
                      await uploadFile(
                        file,
                        selectedCategoryID > 0 ? selectedCategoryID : undefined,
                      );
                      message.success('文件已上传');
                      actionRef.current?.reload();
                      return Upload.LIST_IGNORE;
                    }}
                  >
                    <Button type="primary" icon={<UploadOutlined />}>
                      上传文件
                    </Button>
                  </Upload>,
                  <Button
                    key="import"
                    icon={<LinkOutlined />}
                    onClick={() => setImportOpen(true)}
                  >
                    导入URL
                  </Button>,
                ]
              : []
          }
        />
      </div>
      <ModalForm<{ name?: string; url: string; category_id?: number }>
        key="import-url"
        title="导入URL"
        open={importOpen}
        onOpenChange={setImportOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={{
          category_id: selectedCategoryID > 0 ? selectedCategoryID : undefined,
        }}
        onFinish={async (values) => {
          await importFileURL({
            name: values.name?.trim() || undefined,
            url: values.url.trim(),
            category_id: values.category_id,
          });
          message.success('URL已导入');
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText
          name="name"
          label="名称"
          fieldProps={{ maxLength: 180 }}
          rules={[{ max: 180 }]}
        />
        <ProFormSelect
          name="category_id"
          label="分类"
          options={categoryOptions.map((category) => ({
            label: category.label,
            value: category.id,
          }))}
          fieldProps={{ allowClear: true }}
        />
        <ProFormText
          name="url"
          label="URL"
          fieldProps={{ maxLength: 2048 }}
          rules={[
            { required: true, message: '请输入URL' },
            { type: 'url', message: '请输入有效URL' },
            { max: 2048, message: 'URL不能超过2048个字符' },
          ]}
        />
      </ModalForm>
      <ModalForm<{ name: string; parent_id?: number }>
        key={editingCategory?.id ?? 'create-category'}
        title={editingCategory ? '编辑分类' : '新增分类'}
        open={categoryOpen}
        onOpenChange={setCategoryOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={
          editingCategory ?? {
            name: '',
            parent_id: selectedCategoryID > 0 ? selectedCategoryID : undefined,
          }
        }
        onFinish={async (values) => {
          const body = {
            name: values.name.trim(),
            parent_id: values.parent_id ?? 0,
          };
          if (editingCategory) {
            await updateFileCategory(editingCategory.id, body);
            message.success('分类已更新');
          } else {
            await createFileCategory(body);
            message.success('分类已创建');
          }
          await loadCategories();
          return true;
        }}
      >
        <ProFormText
          name="name"
          label="名称"
          fieldProps={{ maxLength: 80 }}
          rules={[
            { required: true, message: '请输入分类名称' },
            { whitespace: true, message: '分类名称不能只包含空白' },
            { max: 80, message: '分类名称不能超过80个字符' },
          ]}
        />
        <ProFormSelect
          name="parent_id"
          label="父级分类"
          options={categoryOptions
            .filter((category) => category.id !== editingCategory?.id)
            .map((category) => ({
              label: category.label,
              value: category.id,
            }))}
          fieldProps={{ allowClear: true }}
        />
      </ModalForm>
      <ModalForm<{ name: string }>
        key={renamingFile?.id ?? 'idle'}
        title="重命名文件"
        open={renameOpen}
        onOpenChange={(open) => {
          setRenameOpen(open);
          if (!open) {
            setRenamingFile(undefined);
          }
        }}
        modalProps={{ destroyOnHidden: true }}
        initialValues={{ name: renamingFile?.name ?? '' }}
        onFinish={async (values) => {
          if (!renamingFile) {
            return true;
          }
          await renameFile(renamingFile.id, values.name.trim());
          message.success('文件已重命名');
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText
          name="name"
          label="文件名"
          fieldProps={{ maxLength: 180 }}
          rules={[
            { required: true, message: '请输入文件名' },
            { whitespace: true, message: '文件名不能只包含空白' },
            { max: 180, message: '文件名不能超过180个字符' },
          ]}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default Files;
