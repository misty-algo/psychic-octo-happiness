import { describe, expect, it } from 'vitest';

import config from '../config/config';

describe('config', () => {
  it('keeps ProLayout menu locale enabled for ant-design-pro route names', () => {
    const layout = config.layout as { locale?: boolean };

    expect(layout.locale).toBe(true);
  });
});
