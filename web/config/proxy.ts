const apiTarget = process.env.ECHO_ADMIN_WEB_API_TARGET || 'http://127.0.0.1:9322';

/**
 * Local development proxy settings.
 *
 * Production keeps API calls same-origin. The dev proxy lets the Umi dev server
 * talk to a separately running Go API without changing frontend request paths.
 */
export default {
  dev: {
    '/api/': {
      target: apiTarget,
      changeOrigin: true,
    },
  },
  test: {},
  pre: {},
};
