import { CaretDownOutlined, CaretUpOutlined } from '@ant-design/icons';
import { Card } from 'antd';
import type { CardProps } from 'antd/es/card';
import { clsx } from 'clsx';
import { createStyles } from 'antd-style';
import React from 'react';

/**
 * 工作台图表卡片基础组件，移植自 Ant Design Pro 数据分析页的同名组件：
 * ChartCard（指标卡骨架）、Field（footer 键值）、Trend（涨跌指示）、
 * NumberInfo（标题 + 数值 + 子指标）。
 */

const useChartCardStyles = createStyles(({ token }) => ({
  chartCard: {
    position: 'relative',
  },
  chartTop: {
    position: 'relative',
    width: '100%',
    overflow: 'hidden',
  },
  chartTopMargin: {
    marginBottom: '12px',
  },
  metaWrap: {
    float: 'left',
  },
  avatar: {
    position: 'relative',
    top: '4px',
    float: 'left',
    marginRight: '20px',
    img: { borderRadius: '100%' },
  },
  meta: {
    height: '22px',
    color: token.colorTextSecondary,
    fontSize: token.fontSize,
    lineHeight: '22px',
  },
  action: {
    position: 'absolute',
    top: '4px',
    right: '0',
    lineHeight: '1',
    cursor: 'pointer',
  },
  total: {
    height: '38px',
    marginTop: '4px',
    marginBottom: '0',
    overflow: 'hidden',
    color: token.colorTextHeading,
    fontSize: '30px',
    lineHeight: '38px',
    whiteSpace: 'nowrap',
    textOverflow: 'ellipsis',
    wordBreak: 'break-all',
  },
  content: {
    position: 'relative',
    width: '100%',
    marginBottom: '12px',
  },
  contentFixed: {
    position: 'absolute',
    bottom: '0',
    left: '0',
    width: '100%',
  },
  footer: {
    marginTop: '8px',
    paddingTop: '9px',
    borderTop: `1px solid ${token.colorSplit}`,
    '& > *': { position: 'relative' },
  },
  footerMargin: {
    marginTop: '20px',
  },
}));

type ChartCardProps = {
  title: React.ReactNode;
  action?: React.ReactNode;
  total?: React.ReactNode | number | (() => React.ReactNode | number);
  footer?: React.ReactNode;
  contentHeight?: number;
  avatar?: React.ReactNode;
  style?: React.CSSProperties;
} & CardProps;

type TotalType = () => React.ReactNode;

const ChartCardTotal: React.FC<{
  total?: number | TotalType | React.ReactNode;
  totalClassName: string;
}> = ({ total, totalClassName }) => {
  if (!total && total !== 0) {
    return null;
  }
  return <div className={totalClassName}>{typeof total === 'function' ? total() : total}</div>;
};

const ChartCardContent: React.FC<
  ChartCardProps & { styles: Record<string, string> }
> = ({ contentHeight, title, avatar, action, total, footer, children, styles }) => (
  <div className={styles.chartCard}>
    <div
      className={clsx(styles.chartTop, {
        [styles.chartTopMargin]: !children && !footer,
      })}
    >
      <div className={styles.avatar}>{avatar}</div>
      <div className={styles.metaWrap}>
        <div className={styles.meta}>
          <span>{title}</span>
          <span className={styles.action}>{action}</span>
        </div>
        <ChartCardTotal total={total} totalClassName={styles.total} />
      </div>
    </div>
    {children && (
      <div
        className={styles.content}
        style={{
          height: contentHeight || 'auto',
        }}
      >
        <div className={contentHeight ? styles.contentFixed : undefined}>
          {children}
        </div>
      </div>
    )}
    {footer && (
      <div
        className={clsx(styles.footer, {
          [styles.footerMargin]: !children,
        })}
      >
        {footer}
      </div>
    )}
  </div>
);

/** 指标卡骨架：上方标题与总值，中间图表内容区，底部 footer 指标行。 */
const ChartCard: React.FC<ChartCardProps> = (props) => {
  const { styles } = useChartCardStyles();
  const {
    loading = false,
    total: _total,
    contentHeight: _contentHeight,
    action: _action,
    ...cardProps
  } = props;
  return (
    <Card
      loading={loading}
      styles={{
        body: {
          padding: '20px 24px 8px 24px',
        },
      }}
      {...cardProps}
    >
      {loading ? false : <ChartCardContent {...props} styles={styles} />}
    </Card>
  );
};

const useFieldStyles = createStyles(({ token }) => ({
  field: {
    margin: '0',
    overflow: 'hidden',
    whiteSpace: 'nowrap',
    textOverflow: 'ellipsis',
  },
  label: {
    fontSize: token.fontSize,
    lineHeight: '22px',
  },
  number: {
    marginLeft: '8px',
    color: token.colorTextHeading,
  },
}));

/** 指标卡 footer 的「标签 + 数值」行。 */
const Field: React.FC<{
  label: React.ReactNode;
  value: React.ReactNode;
  style?: React.CSSProperties;
}> = ({ label, value, ...rest }) => {
  const { styles } = useFieldStyles();
  return (
    <div className={styles.field} {...rest}>
      <span className={styles.label}>{label}</span>
      <span className={styles.number}>{value}</span>
    </div>
  );
};

const useTrendStyles = createStyles(({ token }) => ({
  trendItem: {
    display: 'inline-block',
    fontSize: token.fontSize,
    lineHeight: '22px',
  },
  up: {
    color: token['red-6'],
  },
  down: {
    top: '-1px',
    color: token['green-6'],
  },
  trendItemGrey: {
    up: { color: token.colorText },
    down: { color: token.colorText },
  },
  reverseColor: {
    up: { color: token['green-6'] },
    down: { color: token['red-6'] },
  },
}));

/** 涨跌趋势指示：数值后跟随红涨绿跌箭头。 */
const Trend: React.FC<{
  colorful?: boolean;
  reverseColor?: boolean;
  flag: 'up' | 'down';
  className?: string;
  title?: string;
  children?: React.ReactNode;
  style?: React.CSSProperties;
}> = ({
  colorful = true,
  reverseColor = false,
  flag,
  children,
  className,
  title = '',
  ...rest
}) => {
  const { styles } = useTrendStyles();
  const classString = clsx(
    styles.trendItem,
    {
      [styles.trendItemGrey]: !colorful,
      [styles.reverseColor]: reverseColor && colorful,
    },
    className,
  );
  return (
    <div {...rest} className={classString} title={title}>
      <span>{children}</span>
      {flag && (
        <span className={styles[flag]}>
          {flag === 'up' ? <CaretUpOutlined /> : <CaretDownOutlined />}
        </span>
      )}
    </div>
  );
};

const useNumberInfoStyles = createStyles(({ token }) => ({
  suffix: {
    marginLeft: '4px',
    color: token.colorText,
    fontSize: '16px',
    fontStyle: 'normal',
  },
  numberInfoTitle: {
    marginBottom: '16px',
    color: token.colorText,
    fontSize: token.fontSizeLG,
    transition: 'all 0.3s',
  },
  numberInfoSubTitle: {
    height: '22px',
    overflow: 'hidden',
    color: token.colorTextSecondary,
    fontSize: token.fontSize,
    lineHeight: '22px',
    whiteSpace: 'nowrap',
    textOverflow: 'ellipsis',
    wordBreak: 'break-all',
  },
  numberInfoValue: {
    marginTop: '4px',
    overflow: 'hidden',
    fontSize: '0',
    whiteSpace: 'nowrap',
    textOverflow: 'ellipsis',
    wordBreak: 'break-all',
    '& > span': { color: token.colorText },
  },
  subTotal: {
    marginRight: '0',
    color: token.colorTextSecondary,
    fontSize: token.fontSizeLG,
    verticalAlign: 'top',
  },
}));

/** 标题 + 主数值 + 涨跌子指标的数字信息块。 */
const NumberInfo: React.FC<{
  title?: React.ReactNode | string;
  subTitle?: React.ReactNode | string;
  renderSubTitle?: () => React.ReactNode;
  total?: React.ReactNode | string;
  status?: 'up' | 'down';
  theme?: string;
  gap?: number;
  subTotal?: number;
  suffix?: string;
  style?: React.CSSProperties;
}> = ({
  theme,
  title,
  subTitle,
  renderSubTitle,
  total,
  subTotal,
  status,
  suffix,
  gap,
  ...rest
}) => {
  const { styles } = useNumberInfoStyles();
  const subTitleNode = renderSubTitle?.() ?? subTitle;
  const hasSubTitle = subTitleNode !== null && subTitleNode !== undefined;
  const themeStyle = theme
    ? (styles as Record<string, string>)[`numberInfo${theme}`]
    : undefined;
  return (
    <div className={themeStyle} {...rest}>
      {title && (
        <div
          className={styles.numberInfoTitle}
          title={typeof title === 'string' ? title : ''}
        >
          {title}
        </div>
      )}
      {hasSubTitle && (
        <div
          className={styles.numberInfoSubTitle}
          title={typeof subTitleNode === 'string' ? subTitleNode : ''}
        >
          {subTitleNode}
        </div>
      )}
      <div
        className={styles.numberInfoValue}
        style={
          gap
            ? {
                marginTop: gap,
              }
            : {}
        }
      >
        <span>
          {total}
          {suffix && <em className={styles.suffix}>{suffix}</em>}
        </span>
        {(status || subTotal) && (
          <span className={styles.subTotal}>
            {subTotal}
            {status && status === 'up' ? (
              <CaretUpOutlined />
            ) : (
              <CaretDownOutlined />
            )}
          </span>
        )}
      </div>
    </div>
  );
};

export { ChartCard, Field, NumberInfo, Trend };
