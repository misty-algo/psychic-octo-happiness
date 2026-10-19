import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@umijs/max', () => ({
  request: vi.fn(),
}));

describe('admin table adapters', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('pageParams', () => {
    it('maps ProTable request params to backend pagination', async () => {
      const { pageParams } = await import('./admin');
      expect(pageParams({ current: 3, pageSize: 50 })).toEqual({
        page: 3,
        page_size: 50,
      });
    });

    it('falls back to first page with 20 rows when params are missing', async () => {
      const { pageParams } = await import('./admin');
      expect(pageParams({})).toEqual({ page: 1, page_size: 20 });
    });
  });

  describe('toTableResult', () => {
    it('maps an envelope response to the ProTable result shape', async () => {
      const { toTableResult } = await import('./admin');
      const result = await toTableResult({
        code: 0,
        message: 'ok',
        data: [{ id: 1 }, { id: 2 }],
        page: { page: 1, page_size: 20, total: 2, has_next: false },
      });
      expect(result).toEqual({
        data: [{ id: 1 }, { id: 2 }],
        success: true,
        total: 2,
      });
    });

    it('reports total 0 when the envelope has no page meta', async () => {
      const { toTableResult } = await import('./admin');
      const result = await toTableResult({
        code: 0,
        message: 'ok',
        data: [],
      });
      expect(result).toEqual({ data: [], success: true, total: 0 });
    });
  });
});
