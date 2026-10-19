# Domain Docs

本文档说明工程技能在探索本仓库时如何读取 domain documentation。

## 布局

本仓库使用 single-context 布局：

- 根目录的 `CONTEXT.md` 定义项目领域语言、业务概念和边界。
- 根目录的 `docs/adr/` 保存项目架构决策。

## 探索前优先读取

- `CONTEXT.md`
- 与当前任务相关的 `docs/adr/` 文档

如果文件不存在，静默继续。不要把缺失的 domain docs 当作当前任务失败，也不要在普通开发任务中主动创建。需要固化新术语或决策时，使用 `domain-modeling` 流程按需更新。

## 文件结构

```text
/
├── CONTEXT.md
└── docs/
    └── adr/
        ├── 0001-*.md
        └── 0002-*.md
```

## 使用 glossary 的词汇

当输出中命名领域概念，例如 issue 标题、重构建议、诊断假设或测试名称时，优先使用 `CONTEXT.md` 中定义的术语，不要使用 glossary 明确避免的同义词。

如果需要的概念尚未进入 glossary，先判断是否正在引入项目不使用的语言。如果确实存在领域语言缺口，在结果中指出可以通过 `domain-modeling` 固化。

## 标出 ADR 冲突

如果建议或实现会违反现有 ADR，必须明确指出，不要静默覆盖。
