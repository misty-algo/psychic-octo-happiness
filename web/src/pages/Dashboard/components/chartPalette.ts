/**
 * Dashboard 图表品牌色。@ant-design/plots 有独立的默认色板，
 * 不跟随 antd 的 colorPrimary，必须在此显式注入；色值与全站主题同源。
 */

/** 单系列图（柱/面积）的主色，等于主题 colorPrimary。 */
export const chartPrimary = '#1677ff';

/** 多类目色板：蓝色主族，尾部金色用于类目区分。 */
export const chartPalette = [
  '#1677ff',
  '#4096ff',
  '#69b1ff',
  '#91caff',
  '#faad14',
  '#ffd666',
];

/** 双系列（成功/失败）对比色：蓝与金。 */
export const chartContrast = ['#1677ff', '#faad14'];

/** 进度环的底轨色与填充色。 */
export const chartRing = ['#e6f4ff', '#1677ff'];
