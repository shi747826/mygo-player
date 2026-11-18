(function () {
  'use strict';

  class ApiError extends Error {
    constructor(message, status) {
      super(message || '请求失败');
      this.name = 'ApiError';
      this.status = Number(status) || 0;
    }
  }

  function token() {
    var config = window.__MYGO_MOBILE__;
    return config && typeof config.token === 'string' ? config.token : '';
  }

  function authHeaders(extra) {
    var headers = Object.assign({ 'X-MyGo-Mobile': token() }, extra || {});
    return headers;
  }

  async function request(path, options) {
    options = options || {};
    if (!token()) {
      throw new ApiError('页面缺少本机会话令牌，请从 MyGo 安卓应用内重新打开。', 401);
    }
    var init = {
      method: options.method || 'GET',
      headers: authHeaders(options.headers),
      cache: 'no-store',
      credentials: 'same-origin',
      signal: options.signal
    };
    if (options.body !== undefined) {
      init.headers['Content-Type'] = 'application/json; charset=utf-8';
      init.body = JSON.stringify(options.body);
    }
    if (options.keepalive) init.keepalive = true;

    var response;
    try {
      response = await fetch(path, init);
    } catch (error) {
      if (error && error.name === 'AbortError') throw error;
      throw new ApiError('无法连接本机来源服务，请检查应用是否仍在运行。', 0);
    }

    var text = await response.text();
    var payload = null;
    if (text) {
      try {
        payload = JSON.parse(text);
      } catch (_error) {
        throw new ApiError('来源服务返回了无法识别的数据。', response.status);
      }
    }
    if (!response.ok) {
      var message = payload && typeof payload.error === 'string' && payload.error
        ? payload.error
        : '来源服务请求失败（HTTP ' + response.status + '）。';
      throw new ApiError(message, response.status);
    }
    return payload;
  }

  function sleep(ms) {
    return new Promise(function (resolve) { setTimeout(resolve, ms); });
  }

  async function waitForBridge(timeoutMs) {
    if (window.mygo && typeof window.mygo.call === 'function') return window.mygo;
    var deadline = Date.now() + (timeoutMs || 12000);
    while (Date.now() < deadline) {
      await sleep(120);
      if (window.mygo && typeof window.mygo.call === 'function') return window.mygo;
    }
    throw new ApiError('没有检测到 MyGo 安卓运行环境。请从应用内打开本页面，而不是普通浏览器。', 0);
  }

  async function preparePlayback() {
    var bridge = await waitForBridge();
    var result;
    try {
      result = await bridge.call('Mobile.PreparePlayback');
    } catch (error) {
      throw new ApiError('无法启用本机视频播放：' + (error && error.message ? error.message : '未知错误'), 0);
    }
    if (result !== true) {
      throw new ApiError('本机视频播放未就绪，Android WebView 没有启用本地媒体访问。', 0);
    }
    return true;
  }

  function sourceQuery(sourceId) {
    return '/api/library?source=' + encodeURIComponent(sourceId);
  }

  window.MyGoApi = Object.freeze({
    ApiError: ApiError,
    preparePlayback: preparePlayback,
    waitForBridge: waitForBridge,
    // Covers load as <img> subresources, so the proxy takes the session in the
    // URL. It also converts formats the WebView cannot decode.
    coverUrl: function (raw) {
      if (typeof raw !== 'string' || !raw) return '';
      return '/api/cover?k=' + encodeURIComponent(token()) + '&u=' + encodeURIComponent(raw);
    },
    sources: function (signal) {
      return request('/api/sources', { signal: signal });
    },
    call: function (source, method, params, signal) {
      return request('/api/call', {
        method: 'POST',
        signal: signal,
        body: { source: source, method: method, params: params || {} }
      });
    },
    release: function (source, sessionId) {
      if (!source || !sessionId) return Promise.resolve(null);
      return request('/api/call', {
        method: 'POST',
        keepalive: true,
        body: { source: source, method: 'Release', params: { SessionID: sessionId } }
      });
    },
    library: function (source, signal) {
      return request(sourceQuery(source), { signal: signal });
    },
    favorite: function (source, series, signal) {
      return request('/api/favorite', {
        method: 'POST',
        signal: signal,
        body: { source: source, series: series }
      });
    },
    progress: function (source, series, episode, position, options) {
      options = options || {};
      return request('/api/progress', {
        method: 'POST',
        signal: options.signal,
        keepalive: !!options.keepalive,
        body: {
          source: source,
          series: series,
          episode: episode,
          position: Number.isFinite(position) ? Math.max(0, position) : 0
        }
      });
    }
  });
})();
