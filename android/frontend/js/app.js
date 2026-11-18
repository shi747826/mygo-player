(function () {
  'use strict';

  var api = window.MyGoApi;
  var RANGE_SIZE = 30;
  var SAVE_INTERVAL = 5000;

  var state = {
    sources: [],
    sourcesLoaded: false,
    sourcesError: '',
    source: null,
    runtime: '',
    backendVersion: '',
    tab: 'discover',
    epoch: 0,
    feedAbort: null,
    feedSeq: 0,
    feedLoading: false,
    loadingMore: false,
    feedError: '',
    pageError: '',
    items: [],
    cursor: '',
    hasMore: false,
    query: '',
    searchInput: '',
    nav: '',
    navLabel: '',
    sort: '',
    filterLabel: '',
    library: null,
    libraryLoading: false,
    libraryError: '',
    libraryAbort: null,
    librarySeq: 0,
    libraryTab: 'history',
    sheetOpen: false,
    pendingSheetAction: null,
    bootAbort: null,
    bootSeq: 0,
    toastTimer: 0,
    lastFocused: null
  };

  var player = {
    open: false,
    sourceId: '',
    sourceName: '',
    series: null,
    episodes: [],
    index: -1,
    detailEpoch: 0,
    detailAbort: null,
    detailLoading: false,
    detailError: '',
    mediaEpoch: 0,
    mediaAbort: null,
    session: '',
    hls: null,
    mediaLoading: false,
    mediaError: '',
    resumePosition: 0,
    resumeHint: null,
    pendingSeek: 0,
    autoPlayPending: false,
    suppressMediaError: false,
    lastSaveAt: 0,
    favorite: false,
    favoriteBusy: false,
    quality: '',
    closing: false,
    seeking: false,
    pointer: null,
    swipedAt: 0,
    pauseTimer: 0,
    endTimer: 0,
    returnFocus: null
  };

  var dom = {};

  function byId(id) { return document.getElementById(id); }

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  function icon(name, className) {
    var svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('class', 'icon' + (className ? ' ' + className : ''));
    svg.setAttribute('aria-hidden', 'true');
    var use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
    use.setAttribute('href', '#i-' + name);
    svg.appendChild(use);
    return svg;
  }

  function setIcon(target, name) {
    var use = target.querySelector('use');
    if (use) use.setAttribute('href', '#i-' + name);
  }

  function clear(node) {
    while (node.firstChild) node.removeChild(node.firstChild);
  }

  function setView(nodes) {
    clear(dom.view);
    var fragment = document.createDocumentFragment();
    (Array.isArray(nodes) ? nodes : [nodes]).forEach(function (node) {
      if (node) fragment.appendChild(node);
    });
    dom.view.appendChild(fragment);
  }

  function append(parent) {
    for (var i = 1; i < arguments.length; i++) {
      var child = arguments[i];
      if (child) parent.appendChild(child);
    }
    return parent;
  }

  function asString(value) {
    return typeof value === 'string' ? value : '';
  }

  function asInt(value) {
    var number = typeof value === 'number' ? value : parseInt(value, 10);
    return Number.isFinite(number) && number > 0 ? Math.floor(number) : 0;
  }

  function asPosition(value) {
    var number = typeof value === 'number' ? value : parseFloat(value);
    return Number.isFinite(number) && number > 0 ? number : 0;
  }

  function safeHttpUrl(value) {
    if (typeof value !== 'string' || !value.trim()) return '';
    try {
      var parsed = new URL(value.trim(), window.location.href);
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return '';
      return parsed.href;
    } catch (_error) {
      return '';
    }
  }

  function isHlsUrl(value) {
    try {
      return /\.m3u8$/i.test(new URL(value).pathname);
    } catch (_error) {
      return false;
    }
  }

  function errorText(error, fallback) {
    if (error && typeof error.message === 'string' && error.message.trim()) return error.message;
    return fallback || '请求失败，请稍后重试。';
  }

  function sourceStorageGet() {
    try { return window.localStorage.getItem('mygo.mobile.source'); } catch (_error) { return ''; }
  }

  function sourceStorageSet(id) {
    try { window.localStorage.setItem('mygo.mobile.source', id); } catch (_error) { /* private mode */ }
  }

  function normalizeSeries(raw, fallbackId) {
    if (!raw || typeof raw !== 'object') return null;
    var id = asString(raw.id || raw.ID || fallbackId).trim();
    if (!id) return null;
    return {
      id: id,
      title: asString(raw.title || raw.Title) || '未命名剧集',
      cover: asString(raw.cover || raw.Cover),
      description: asString(raw.description || raw.Description),
      episodes: asInt(raw.episodes !== undefined ? raw.episodes : raw.Episodes),
      release_state: asString(raw.release_state || raw.ReleaseState),
      release_note: asString(raw.release_note || raw.ReleaseNote)
    };
  }

  function normalizeEpisode(raw) {
    if (!raw || typeof raw !== 'object') return null;
    var id = asString(raw.id || raw.ID).trim();
    if (!id) return null;
    return {
      id: id,
      title: asString(raw.title || raw.Title),
      number: asInt(raw.number !== undefined ? raw.number : raw.Number)
    };
  }

  function normalizeFilters(raw) {
    var value = raw;
    if (typeof value === 'string') {
      try { value = JSON.parse(value); } catch (_error) { value = []; }
    }
    if (!Array.isArray(value)) return [];
    return value.map(function (group) {
      if (!group || typeof group !== 'object') return null;
      var options = Array.isArray(group.options) ? group.options : [];
      return {
        label: asString(group.label) || '筛选',
        value: asString(group.value),
        options: options.map(function (option) {
          if (!option || typeof option !== 'object') return null;
          return { label: asString(option.label) || '未命名', value: asString(option.value) };
        }).filter(Boolean)
      };
    }).filter(function (group) { return group && group.options.length > 0; });
  }

  function normalizeSource(raw) {
    if (!raw || typeof raw !== 'object') return null;
    var id = asString(raw.id).trim();
    if (!id) return null;
    var navigation = Array.isArray(raw.navigation) ? raw.navigation : [];
    var capabilities = Array.isArray(raw.capabilities) ? raw.capabilities : [];
    return {
      id: id,
      name: asString(raw.name) || id,
      version: asString(raw.version),
      capabilities: capabilities.map(asString).filter(Boolean),
      navigation: navigation.map(function (item) {
        if (!item || typeof item !== 'object') return null;
        var label = asString(item.label);
        var value = asString(item.value);
        return label ? { label: label, value: value } : null;
      }).filter(Boolean),
      filters: normalizeFilters(raw.filters),
      available: raw.available === true,
      error: asString(raw.error)
    };
  }

  function normalizePage(payload) {
    if (!payload || typeof payload !== 'object') {
      return { items: [], cursor: '', hasMore: false };
    }
    var rawItems = Array.isArray(payload.Items) ? payload.Items
      : (Array.isArray(payload.items) ? payload.items : []);
    var items = rawItems.map(function (item) { return normalizeSeries(item); }).filter(Boolean);
    return {
      items: items,
      cursor: asString(payload.Cursor !== undefined ? payload.Cursor : payload.cursor),
      hasMore: payload.HasMore === true || payload.has_more === true
    };
  }

  function normalizeLibrary(payload) {
    var library = {
      favorites: Object.create(null),
      history: Object.create(null)
    };
    if (!payload || typeof payload !== 'object') return library;
    var favorites = payload.favorites && typeof payload.favorites === 'object' ? payload.favorites : {};
    Object.keys(favorites).forEach(function (key) {
      var series = normalizeSeries(favorites[key], key);
      if (series) library.favorites[series.id || key] = series;
    });
    var history = payload.history && typeof payload.history === 'object' ? payload.history : {};
    Object.keys(history).forEach(function (key) {
      var entry = history[key];
      if (!entry || typeof entry !== 'object') return;
      var series = normalizeSeries(entry.series, key);
      var episode = normalizeEpisode(entry.episode);
      if (!series || !episode) return;
      library.history[series.id || key] = {
        series: series,
        episode: episode,
        position: asPosition(entry.position),
        updated_at: asString(entry.updated_at)
      };
    });
    return library;
  }

  function currentLibrary() {
    if (!state.library) return null;
    return state.library;
  }

  function episodeLabel(episode, index) {
    var number = episode && episode.number ? episode.number : index + 1;
    return '第 ' + number + ' 集';
  }

  function episodeNumber(episode, index) {
    return episode && episode.number ? String(episode.number) : String(index + 1);
  }

  function episodeTitle(episode, index) {
    if (episode && episode.title) return episode.title;
    return episodeLabel(episode, index);
  }

  function formatTime(seconds) {
    if (!Number.isFinite(seconds) || seconds < 0) return '0:00';
    var total = Math.floor(seconds);
    var hours = Math.floor(total / 3600);
    var minutes = Math.floor((total % 3600) / 60);
    var rest = total % 60;
    if (hours > 0) return hours + ':' + String(minutes).padStart(2, '0') + ':' + String(rest).padStart(2, '0');
    return minutes + ':' + String(rest).padStart(2, '0');
  }

  function showToast(message, duration) {
    if (!dom.toast) return;
    dom.toast.textContent = String(message || '');
    dom.toast.classList.add('is-visible');
    window.clearTimeout(state.toastTimer);
    state.toastTimer = window.setTimeout(function () {
      dom.toast.classList.remove('is-visible');
    }, duration || 2300);
  }

  function sourceName() {
    return state.source ? state.source.name : '';
  }

  function buildPageHead() {
    var head = el('header', 'page-head');
    var wordmark = el('div', 'wordmark');
    wordmark.appendChild(document.createTextNode('MyGo'));
    wordmark.appendChild(el('i'));
    var pill = el('button', 'source-pill');
    pill.type = 'button';
    pill.setAttribute('aria-label', '选择来源');
    var dot = el('i', 'source-dot' + (state.source ? '' : ' is-off'));
    var name = el('span', '', state.source ? state.source.name : (state.sourcesLoaded ? '无可用来源' : '连接中'));
    append(pill, dot, name, icon('down'));
    pill.addEventListener('click', openSourceSheet);
    append(head, wordmark, pill);
    return head;
  }

  function stateBlock(options) {
    var block = el('section', 'state-block');
    var badge = el('div', 'state-icon');
    if (options.spinner) {
      badge.appendChild(el('div', 'spinner'));
    } else {
      badge.appendChild(icon(options.icon || 'info'));
    }
    var title = el('h3', '', options.title || '暂时没有内容');
    var message = el('p', '', options.message || '');
    block.append(badge, title, message);
    if (options.actionLabel && options.onAction) {
      var action = el('button', 'button', options.actionLabel);
      action.type = 'button';
      action.addEventListener('click', options.onAction);
      block.appendChild(action);
    }
    return block;
  }

  function skeletonFeed() {
    var wrap = el('div', 'skeleton-grid');
    wrap.setAttribute('aria-hidden', 'true');
    for (var col = 0; col < 2; col++) {
      var column = el('div', 'skeleton-col');
      for (var i = 0; i < 3; i++) {
        var card = el('div', 'skeleton-card');
        append(card, el('div', 'skeleton-cover'), el('div', 'skeleton-line'), el('div', 'skeleton-line short'));
        column.appendChild(card);
      }
      wrap.appendChild(column);
    }
    return wrap;
  }

  function fillCover(container, series, decorative) {
    var url = safeHttpUrl(series.cover);
    if (url && api.coverUrl) url = api.coverUrl(url);
    if (!url) {
      var placeholder = el('div', 'cover-placeholder');
      placeholder.setAttribute('aria-hidden', 'true');
      container.appendChild(placeholder);
      return;
    }
    var image = el('img');
    image.src = url;
    image.alt = decorative ? '' : (series.title + ' 封面');
    image.loading = 'lazy';
    image.decoding = 'async';
    image.referrerPolicy = 'no-referrer';
    image.draggable = false;
    image.addEventListener('error', function () {
      if (image.parentNode) image.parentNode.removeChild(image);
      var fallback = el('div', 'cover-placeholder');
      fallback.setAttribute('aria-hidden', 'true');
      container.appendChild(fallback);
    }, { once: true });
    container.appendChild(image);
  }

  function buildCard(series, index) {
    var card = el('button', 'card' + (index % 3 === 1 ? ' tall' : ''));
    card.type = 'button';
    card.style.animationDelay = Math.min(index, 8) * 18 + 'ms';
    card.setAttribute('aria-label', series.title + (series.episodes ? '，' + series.episodes + ' 集' : '') + (series.release_state === 'upcoming' ? '，待播' : ''));

    var cover = el('span', 'cover');
    fillCover(cover, series, false);
    if (series.episodes > 0) {
      cover.appendChild(el('span', 'episode-badge', series.episodes + ' 集'));
    }
    if (series.release_state === 'upcoming') {
      cover.appendChild(el('span', 'state-badge', '待播'));
    }

    var meta = '';
    if (series.release_state === 'upcoming') {
      meta = series.release_note || '待播';
    } else {
      meta = series.description || series.release_note || sourceName();
    }

    var title = el('span', 'card-title', series.title);
    var metaNode = el('span', 'card-meta', meta || ' ');
    append(card, cover, title, metaNode);
    card.addEventListener('click', function () { openSeries(series); });
    return card;
  }

  function buildFeed(items) {
    var grid = el('div', 'feed-grid');
    var columns = [el('div', 'feed-column'), el('div', 'feed-column')];
    items.forEach(function (series, index) {
      columns[index % 2].appendChild(buildCard(series, index));
    });
    grid.append(columns[0], columns[1]);
    return grid;
  }

  function buildPager() {
    var pager = el('div', 'pager');
    if (state.pageError) {
      var errorBox = el('div', 'inline-error');
      errorBox.appendChild(document.createTextNode(state.pageError));
      var retry = el('button', 'button button-secondary', '重试加载');
      retry.type = 'button';
      retry.addEventListener('click', function () { loadFeed(true); });
      errorBox.appendChild(retry);
      pager.appendChild(errorBox);
      return pager;
    }
    if (state.loadingMore) {
      var busy = el('button', 'button button-secondary');
      busy.type = 'button';
      busy.disabled = true;
      busy.appendChild(el('span', 'spinner'));
      busy.appendChild(document.createTextNode('正在加载'));
      pager.appendChild(busy);
      return pager;
    }
    if (state.hasMore) {
      var more = el('button', 'button button-secondary', '加载更多');
      more.type = 'button';
      more.addEventListener('click', function () { loadFeed(true); });
      pager.appendChild(more);
      return pager;
    }
    if (state.items.length > 0) {
      pager.appendChild(el('p', 'feed-end', '没有更多了'));
    }
    return pager;
  }

  function buildSearchForm() {
    var form = el('form', 'search-form');
    form.setAttribute('role', 'search');
    form.appendChild(icon('search'));
    var input = el('input');
    input.type = 'search';
    input.name = 'q';
    input.placeholder = '搜索当前来源的剧集';
    input.autocomplete = 'off';
    input.enterKeyHint = 'search';
    input.maxLength = 80;
    input.value = state.searchInput;
    input.setAttribute('aria-label', '搜索剧集');
    input.addEventListener('input', function () { state.searchInput = input.value; });
    form.appendChild(input);

    if (state.searchInput || state.query) {
      var clearButton = el('button', 'search-clear');
      clearButton.type = 'button';
      clearButton.setAttribute('aria-label', '清除搜索');
      clearButton.appendChild(icon('close'));
      clearButton.addEventListener('click', clearSearch);
      form.appendChild(clearButton);
    }

    var submit = el('button', 'search-submit');
    submit.type = 'submit';
    submit.setAttribute('aria-label', '搜索');
    submit.appendChild(icon('search'));
    form.appendChild(submit);

    form.addEventListener('submit', function (event) {
      event.preventDefault();
      var query = input.value.trim();
      if (!query) {
        clearSearch();
        return;
      }
      state.query = query;
      state.searchInput = query;
      state.items = [];
      state.cursor = '';
      state.hasMore = false;
      state.feedError = '';
      state.pageError = '';
      dom.mainScroll.scrollTop = 0;
      loadFeed(false);
    });
    return form;
  }

  function selectNav(item) {
    if (state.nav === item.value && !state.query) return;
    state.nav = item.value;
    state.navLabel = item.label;
    state.query = '';
    state.searchInput = '';
    state.items = [];
    state.cursor = '';
    state.hasMore = false;
    state.feedError = '';
    state.pageError = '';
    dom.mainScroll.scrollTop = 0;
    loadFeed(false);
  }

  function clearSearch() {
    state.query = '';
    state.searchInput = '';
    state.items = [];
    state.cursor = '';
    state.hasMore = false;
    state.feedError = '';
    state.pageError = '';
    dom.mainScroll.scrollTop = 0;
    loadFeed(false);
  }

  function renderDiscover() {
    var nodes = [buildPageHead()];
    var intro = el('section', 'discover-intro');
    var introCopy = el('div');
    append(introCopy,
      el('p', 'eyebrow', 'MYGO · ANDROID'),
      el('h1', '', '今天，追点什么'),
      el('p', '', state.source ? '内容由当前来源实时提供。' : '来源就绪后，这里会出现真实内容。')
    );
    var mark = el('div', 'intro-mark');
    mark.appendChild(icon('discover'));
    append(intro, introCopy, mark);
    nodes.push(intro);

    if (!state.source) {
      if (!state.sourcesLoaded) {
        nodes.push(stateBlock({
          spinner: true,
          title: '正在连接本机来源',
          message: '正在读取 APK 内嵌来源与来源导航。'
        }));
        if (state.sourcesError) {
          nodes.push(stateBlock({
            icon: 'info',
            title: '暂时无法读取来源',
            message: state.sourcesError,
            actionLabel: '重新连接',
            onAction: boot
          }));
        }
      } else {
        nodes.push(stateBlock({
          icon: 'source',
          title: state.sources.some(function (source) { return source.available; }) ? '尚未选择来源' : '当前没有可用来源',
          message: '来源名称与可用状态来自本机 manifest。可在“来源”面板查看详情，或重新连接来源服务。',
          actionLabel: '查看来源',
          onAction: openSourceSheet
        }));
        nodes.push(stateBlock({
          icon: 'refresh',
          title: '来源服务',
          message: state.sourcesError || '如果来源刚刚启动，可以重新读取一次。',
          actionLabel: '重新连接',
          onAction: boot
        }));
      }
      setView(nodes);
      return;
    }

    nodes.push(buildSearchForm());
    if (!state.query) {
      var chipScroll = el('div', 'chip-scroll');
      var navItems = state.source.navigation || [];
      navItems.forEach(function (item) {
        var chip = el('button', 'chip' + (state.nav === item.value ? ' is-active' : ''), item.label);
        chip.type = 'button';
        chip.setAttribute('aria-pressed', state.nav === item.value ? 'true' : 'false');
        chip.addEventListener('click', function () { selectNav(item); });
        chipScroll.appendChild(chip);
      });
      if (state.source.filters.length) {
        var filterChip = el('button', 'chip filter-chip' + (state.filterLabel ? ' has-value' : ''));
        filterChip.type = 'button';
        filterChip.setAttribute('aria-label', '打开筛选');
        filterChip.appendChild(icon('list'));
        filterChip.appendChild(document.createTextNode(state.filterLabel || '筛选'));
        filterChip.addEventListener('click', openFilterSheet);
        chipScroll.appendChild(filterChip);
      }
      if (chipScroll.childNodes.length) nodes.push(chipScroll);
    }

    var heading = el('div', 'feed-heading');
    var headingTitle;
    if (state.query) {
      headingTitle = el('h2', '', '搜索「' + state.query + '」');
    } else {
      headingTitle = el('h2', '', state.navLabel || '最新内容');
    }
    heading.appendChild(headingTitle);
    if (state.query) {
      var reset = el('button', 'feed-reset', '返回发现');
      reset.type = 'button';
      reset.addEventListener('click', clearSearch);
      heading.appendChild(reset);
    } else {
      heading.appendChild(el('span', '', state.items.length ? '已加载 ' + state.items.length + ' 部' : '实时内容'));
    }
    nodes.push(heading);

    if (state.feedLoading && state.items.length === 0) {
      nodes.push(skeletonFeed());
    } else if (state.feedError && state.items.length === 0) {
      nodes.push(stateBlock({
        icon: 'refresh',
        title: '内容加载失败',
        message: state.feedError,
        actionLabel: '重新加载',
        onAction: function () { loadFeed(false); }
      }));
    } else if (state.items.length === 0) {
      nodes.push(stateBlock({
        icon: state.query ? 'search' : 'discover',
        title: state.query ? '没有找到相关剧集' : '这个频道暂时没有内容',
        message: state.query ? '换一个关键词，或返回来源的导航继续浏览。' : '可以切换来源导航或筛选条件后再试。',
        actionLabel: state.query ? '返回发现' : '刷新内容',
        onAction: state.query ? clearSearch : function () { loadFeed(false); }
      }));
    } else {
      nodes.push(buildFeed(state.items));
      nodes.push(buildPager());
    }

    setView(nodes);
  }

  async function loadFeed(append) {
    if (!state.source) return;
    if (append && (state.loadingMore || !state.hasMore)) return;
    if (state.feedAbort && !append) state.feedAbort.abort();

    var epoch = state.epoch;
    var sequence = ++state.feedSeq;
    var sourceId = state.source.id;
    var controller = new AbortController();
    state.feedAbort = controller;
    state.feedLoading = !append;
    state.loadingMore = !!append;
    if (!append) state.loadingMore = false;
    state.pageError = '';
    if (!append) {
      state.feedError = '';
      state.items = [];
      state.cursor = '';
      state.hasMore = false;
    }
    renderDiscover();

    var method = state.query ? 'Search' : 'Discover';
    var params;
    if (state.query) {
      params = { Search: { Query: state.query, Cursor: append ? state.cursor : '', PageSize: 20 } };
    } else {
      params = {
        Channel: state.nav,
        Cursor: append ? state.cursor : '',
        Sort: state.sort,
        PageSize: 20
      };
    }

    try {
      var payload = await api.call(sourceId, method, params, controller.signal);
      if (sequence !== state.feedSeq || epoch !== state.epoch || !state.source || state.source.id !== sourceId) return;
      var page = normalizePage(payload);
      if (append) {
        var known = Object.create(null);
        state.items.forEach(function (series) { known[series.id] = true; });
        page.items.forEach(function (series) {
          if (!known[series.id]) {
            known[series.id] = true;
            state.items.push(series);
          }
        });
      } else {
        state.items = page.items;
      }
      state.cursor = page.cursor;
      state.hasMore = page.hasMore;
      if (page.hasMore && !page.cursor) {
        state.hasMore = false;
        state.pageError = '来源没有返回下一页游标，已停止继续加载。';
      }
    } catch (error) {
      if (error && error.name === 'AbortError') return;
      if (sequence !== state.feedSeq || epoch !== state.epoch) return;
      var message = errorText(error, '内容加载失败，请重试。');
      if (append) state.pageError = message;
      else state.feedError = message;
    } finally {
      if (sequence === state.feedSeq && epoch === state.epoch) {
        state.feedLoading = false;
        state.loadingMore = false;
        state.feedAbort = null;
        renderDiscover();
      }
    }
  }

  async function loadLibrary(showLoading) {
    if (!state.source) {
      state.library = null;
      state.libraryLoading = false;
      return;
    }
    if (state.libraryAbort) state.libraryAbort.abort();
    var epoch = state.epoch;
    var sequence = ++state.librarySeq;
    var sourceId = state.source.id;
    var controller = new AbortController();
    state.libraryAbort = controller;
    state.libraryLoading = true;
    state.libraryError = '';
    if (showLoading && state.tab === 'library') renderLibrary();
    try {
      var payload = await api.library(sourceId, controller.signal);
      if (sequence !== state.librarySeq || epoch !== state.epoch || !state.source || state.source.id !== sourceId) return;
      state.library = normalizeLibrary(payload);
      if (player.open && player.sourceId === sourceId && player.series) {
        player.favorite = !!state.library.favorites[player.series.id];
        updateFavoriteButton();
      }
    } catch (error) {
      if (error && error.name === 'AbortError') return;
      if (sequence !== state.librarySeq || epoch !== state.epoch) return;
      state.libraryError = errorText(error, '追剧列表加载失败。');
    } finally {
      if (sequence === state.librarySeq && epoch === state.epoch) {
        state.libraryLoading = false;
        state.libraryAbort = null;
        if (state.tab === 'library') renderLibrary();
        else if (state.tab === 'me') renderMe();
      }
    }
  }

  function historyEntries() {
    var library = currentLibrary();
    if (!library) return [];
    return Object.keys(library.history).map(function (key) { return library.history[key]; })
      .sort(function (a, b) {
        var at = Date.parse(a.updated_at) || 0;
        var bt = Date.parse(b.updated_at) || 0;
        return bt - at;
      });
  }

  function favoriteEntries() {
    var library = currentLibrary();
    if (!library) return [];
    return Object.keys(library.favorites).map(function (key) { return library.favorites[key]; })
      .sort(function (a, b) { return a.title.localeCompare(b.title, 'zh-CN'); });
  }

  function buildLibraryRow(series, detail, options) {
    options = options || {};
    var row = el('button', 'library-row');
    row.type = 'button';
    var cover = el('span', 'library-cover');
    fillCover(cover, series, true);
    var copy = el('span', 'library-copy');
    copy.appendChild(el('h3', '', series.title));
    copy.appendChild(el('p', '', detail));
    append(row, cover, copy, icon('chevron'));
    row.addEventListener('click', function () {
      openSeries(series, options.openOptions || {});
    });
    return row;
  }

  function buildResumeCard(entry) {
    var card = el('button', 'resume-card');
    card.type = 'button';
    var cover = el('span', 'resume-cover');
    fillCover(cover, entry.series, true);
    var copy = el('div', 'resume-copy');
    copy.appendChild(el('p', 'eyebrow', '继续观看'));
    copy.appendChild(el('h3', '', entry.series.title));
    var detail = episodeLabel(entry.episode, 0);
    if (entry.position > 0) detail += ' · ' + formatTime(entry.position);
    copy.appendChild(el('p', '', detail));
    var play = el('span', 'resume-play');
    play.appendChild(icon('play'));
    append(card, cover, copy, play);
    card.addEventListener('click', function () {
      openSeries(entry.series, { episodeId: entry.episode.id, position: entry.position });
    });
    return card;
  }

  function renderLibrary() {
    var nodes = [buildPageHead()];
    var title = el('header', 'page-title');
    append(title,
      el('p', 'eyebrow', 'YOUR LIBRARY'),
      el('h1', '', '追剧'),
      el('p', '', '收藏与进度按来源分别保存，只留在这台设备。')
    );
    nodes.push(title);

    var tabs = el('div', 'segmented');
    [
      { id: 'history', label: '观看历史' },
      { id: 'favorites', label: '我的收藏' }
    ].forEach(function (item) {
      var button = el('button', item.id === state.libraryTab ? 'is-active' : '', item.label);
      button.type = 'button';
      button.setAttribute('aria-pressed', item.id === state.libraryTab ? 'true' : 'false');
      button.addEventListener('click', function () {
        state.libraryTab = item.id;
        renderLibrary();
      });
      tabs.appendChild(button);
    });
    nodes.push(tabs);

    if (!state.source) {
      nodes.push(stateBlock({
        icon: 'source',
        title: '还没有选择来源',
        message: '追剧列表依赖当前来源。先选择一个可用来源。',
        actionLabel: '选择来源',
        onAction: openSourceSheet
      }));
      setView(nodes);
      return;
    }

    var note = el('p', 'library-source-note');
    note.appendChild(el('i', 'source-dot'));
    note.appendChild(document.createTextNode('当前来源 · ' + state.source.name));
    nodes.push(note);

    if (state.libraryLoading && !state.library) {
      nodes.push(stateBlock({ spinner: true, title: '正在读取追剧列表', message: '收藏与播放进度来自本机来源存储。' }));
      setView(nodes);
      return;
    }
    if (state.libraryError) {
      nodes.push(stateBlock({
        icon: 'refresh',
        title: '追剧列表加载失败',
        message: state.libraryError,
        actionLabel: '重新加载',
        onAction: function () { loadLibrary(true); }
      }));
      setView(nodes);
      return;
    }

    if (state.libraryTab === 'history') {
      var entries = historyEntries();
      if (!entries.length) {
        nodes.push(stateBlock({
          icon: 'library',
          title: '还没有观看记录',
          message: '播放后会自动保存进度，下次可以从这里继续。',
          actionLabel: '去发现',
          onAction: function () { switchTab('discover'); }
        }));
      } else {
        nodes.push(buildResumeCard(entries[0]));
        if (entries.length > 1) {
          nodes.push(el('p', 'library-section-label', '最近观看'));
          var list = el('div', 'library-list');
          entries.slice(1).forEach(function (entry) {
            var detail = episodeLabel(entry.episode, 0);
            if (entry.position > 0) detail += ' · 播放到 ' + formatTime(entry.position);
            list.appendChild(buildLibraryRow(entry.series, detail, {
              openOptions: { episodeId: entry.episode.id, position: entry.position }
            }));
          });
          nodes.push(list);
        }
      }
    } else {
      var favorites = favoriteEntries();
      if (!favorites.length) {
        nodes.push(stateBlock({
          icon: 'heart',
          title: '还没有收藏',
          message: '在播放页点亮爱心，剧集会出现在当前来源的追剧列表中。',
          actionLabel: '去发现',
          onAction: function () { switchTab('discover'); }
        }));
      } else {
        var favoriteList = el('div', 'library-list');
        favorites.forEach(function (series) {
          var detail = series.episodes ? '共 ' + series.episodes + ' 集' : (series.release_note || '已加入追剧');
          favoriteList.appendChild(buildLibraryRow(series, detail));
        });
        nodes.push(favoriteList);
      }
    }
    setView(nodes);
  }

  function buildSettingRow(options) {
    var row = el('button', 'setting-row');
    row.type = 'button';
    var badge = el('span', 'setting-icon' + (options.accent ? ' accent' : ''));
    badge.appendChild(icon(options.icon));
    var copy = el('span', 'setting-copy');
    copy.appendChild(el('strong', '', options.title));
    var small = el('small', '', options.detail);
    if (options.detailTitle) small.title = options.detailTitle;
    copy.appendChild(small);
    append(row, badge, copy, icon('chevron'));
    if (options.onClick) row.addEventListener('click', options.onClick);
    return row;
  }

  function renderMe() {
    var nodes = [buildPageHead()];
    var title = el('header', 'page-title');
    append(title, el('p', 'eyebrow', 'THIS DEVICE'), el('h1', '', '我的'));
    nodes.push(title);

    var library = currentLibrary();
    var historyCount = library ? Object.keys(library.history).length : 0;
    var favoriteCount = library ? Object.keys(library.favorites).length : 0;
    var availableCount = state.sources.filter(function (source) { return source.available; }).length;

    var profile = el('section', 'profile-card');
    profile.appendChild(el('div', 'profile-avatar', 'M'));
    var profileCopy = el('div', 'profile-copy');
    profileCopy.appendChild(el('h2', '', '本机空间'));
    profileCopy.appendChild(el('p', '', '收藏、来源与进度保存在这台设备，不需要账号登录。'));
    profile.appendChild(profileCopy);
    var privateMark = el('span', 'private-mark');
    privateMark.appendChild(icon('info'));
    privateMark.appendChild(document.createTextNode('仅本机'));
    profile.appendChild(privateMark);
    nodes.push(profile);

    var stats = el('div', 'stat-card');
    var statItems = [
      { value: favoriteCount, label: '收藏', onClick: function () { state.libraryTab = 'favorites'; switchTab('library'); } },
      { value: historyCount, label: '观看历史', onClick: function () { state.libraryTab = 'history'; switchTab('library'); } },
      { value: availableCount, label: '可用来源', onClick: openSourceSheet }
    ];
    statItems.forEach(function (item) {
      var button = el('button');
      button.type = 'button';
      append(button, el('strong', '', String(item.value)), el('span', '', item.label));
      button.addEventListener('click', item.onClick);
      stats.appendChild(button);
    });
    nodes.push(stats);

    nodes.push(el('p', 'group-label', '来源与播放'));
    var settings = el('section', 'settings-card');
    settings.appendChild(buildSettingRow({
      icon: 'source',
      title: '来源管理',
      detail: state.source ? state.source.name + (state.source.version ? ' · v' + state.source.version : '') : '尚未连接来源',
      accent: true,
      onClick: openSourceSheet
    }));
    settings.appendChild(buildSettingRow({
      icon: 'info',
      title: '来源随 APK 内嵌',
      detail: '手机运行时不能安装桌面 EXE 插件或外部仓库',
      onClick: function () {
        showToast('Android 来源在构建 APK 时内嵌，不能从手机端安装桌面插件。', 3600);
      }
    }));
    settings.appendChild(buildSettingRow({
      icon: 'refresh',
      title: '重新连接来源',
      detail: '重新读取来源列表与可用状态',
      onClick: boot
    }));
    nodes.push(settings);

    var note = el('div', 'note-card');
    note.appendChild(el('strong', '', '关于移动端来源'));
    note.appendChild(document.createTextNode(' 当前 Android 运行时只使用随 APK 内嵌的来源，不能运行桌面版 EXE 插件，也不能在这里添加外部仓库。来源名称、版本、导航和筛选均由后端 manifest 提供。'));
    nodes.push(note);

    var build = el('p', 'build-line');
    build.textContent = 'MyGo Player · 移动来源接口 ' + (state.runtime || '未连接') +
      (state.backendVersion ? ' · v' + state.backendVersion : '');
    nodes.push(build);
    setView(nodes);
  }

  function syncTabbar() {
    Array.prototype.forEach.call(dom.tabbar.querySelectorAll('.tab'), function (button) {
      var active = button.getAttribute('data-tab') === state.tab;
      button.classList.toggle('is-active', active);
      if (active) button.setAttribute('aria-current', 'page');
      else button.removeAttribute('aria-current');
    });
  }

  function render() {
    if (!dom.view) return;
    syncTabbar();
    if (state.tab === 'library') renderLibrary();
    else if (state.tab === 'me') renderMe();
    else renderDiscover();
  }

  function switchTab(tab) {
    if (player.open) return;
    if (tab === 'library') state.libraryTab = state.libraryTab || 'history';
    state.tab = tab;
    dom.mainScroll.scrollTop = 0;
    render();
    if (tab === 'library' && state.source) loadLibrary(true);
  }

  function openSourceSheet() {
    if ((!state.sourcesLoaded || state.sourcesError) && !state.sources.length) {
      openSheet('来源', 'SOURCE RUNTIME', function (body) {
        body.appendChild(stateBlock({
          icon: 'refresh',
          title: state.sourcesLoaded ? '来源列表读取失败' : '来源列表尚未载入',
          message: state.sourcesError || '请重新连接本机来源服务。',
          actionLabel: '重新连接',
          onAction: function () {
            closeSheetWithAction(boot);
          }
        }));
      });
      return;
    }

    openSheet('来源', 'SOURCE RUNTIME', function (body) {
      if (!state.sources.length) {
        var empty = el('p', 'sheet-empty', '没有发现可用的内嵌来源。请确认 APK 构建时已经注入来源。');
        body.appendChild(empty);
        return;
      }
      var list = el('div', 'sheet-list');
      state.sources.forEach(function (source) {
        var row = el('button', 'source-option' + (state.source && state.source.id === source.id ? ' is-active' : ''));
        row.type = 'button';
        row.disabled = !source.available;
        var avatar = el('span', 'source-avatar', Array.from(source.name)[0] || 'M');
        var copy = el('span', 'source-copy');
        var strong = el('strong');
        strong.appendChild(document.createTextNode(source.name));
        strong.appendChild(el('em', '', source.available ? '可用' : '不可用'));
        copy.appendChild(strong);
        var detail = source.version ? 'v' + source.version : '未知版本';
        if (source.capabilities.length) detail += ' · ' + source.capabilities.length + ' 项能力';
        var small = el('small', source.error ? 'error' : '', source.error || detail);
        copy.appendChild(small);
        append(row, avatar, copy);
        if (state.source && state.source.id === source.id) row.appendChild(icon('check'));
        row.addEventListener('click', function () {
          if (!source.available) return;
          if (state.source && state.source.id === source.id) {
            closeSheetWithAction(function () {});
            return;
          }
          closeSheetWithAction(function () { activateSource(source, true); });
        });
        list.appendChild(row);
      });
      body.appendChild(list);
      body.appendChild(el('p', 'source-note',
        '来源名称、版本、导航与筛选均来自来源 manifest。Android 来源随 APK 内嵌；当前手机运行时不能安装桌面 EXE 插件，也不能添加外部仓库。'));
    });
  }

  function openFilterSheet() {
    var groups = state.source ? state.source.filters : [];
    openSheet('筛选', 'SOURCE FILTERS', function (body) {
      if (!groups.length) {
        body.appendChild(el('p', 'sheet-empty', '当前来源没有提供筛选条件。'));
        return;
      }
      body.appendChild(el('p', 'sheet-sub', '筛选值由当前来源 manifest 提供，并会随发现请求一起提交。'));
      groups.forEach(function (group) {
        var section = el('section', 'filter-group');
        section.appendChild(el('h3', '', group.label));
        var options = el('div', 'filter-options');
        group.options.forEach(function (option) {
          var selected = state.sort === option.value && state.filterLabel === (group.label + '：' + option.label);
          var button = el('button', 'filter-option' + (selected ? ' is-active' : ''), option.label);
          button.type = 'button';
          button.setAttribute('aria-pressed', selected ? 'true' : 'false');
          button.addEventListener('click', function () {
            state.sort = option.value;
            state.filterLabel = group.label + '：' + option.label;
            state.items = [];
            state.cursor = '';
            state.hasMore = false;
            state.feedError = '';
            state.pageError = '';
            closeSheetWithAction(function () {
              dom.mainScroll.scrollTop = 0;
              loadFeed(false);
            });
          });
          options.appendChild(button);
        });
        section.appendChild(options);
        body.appendChild(section);
      });
      if (state.sort || state.filterLabel) {
        var actions = el('div', 'sheet-actions');
        var clearButton = el('button', 'button button-secondary', '清除筛选');
        clearButton.type = 'button';
        clearButton.addEventListener('click', function () {
          state.sort = '';
          state.filterLabel = '';
          state.items = [];
          state.cursor = '';
          state.hasMore = false;
          closeSheetWithAction(function () { loadFeed(false); });
        });
        actions.appendChild(clearButton);
        body.appendChild(actions);
      }
    });
  }

  function openSheet(title, kicker, buildBody) {
    state.lastFocused = document.activeElement;
    if (!state.sheetOpen) {
      pushHistory('sheet');
      state.sheetOpen = true;
    }
    dom.sheetTitle.textContent = title;
    dom.sheetKicker.textContent = kicker || '';
    clear(dom.sheetBody);
    dom.sheetBody.scrollTop = 0;
    buildBody(dom.sheetBody);
    dom.overlay.hidden = false;
    dom.sheet.focus();
  }

  function closeSheetUI() {
    if (!state.sheetOpen) return;
    state.sheetOpen = false;
    dom.overlay.hidden = true;
    clear(dom.sheetBody);
    var target = state.lastFocused;
    state.lastFocused = null;
    if (target && target.isConnected && !player.open) {
      try { target.focus({ preventScroll: true }); } catch (_error) { /* ignore */ }
    }
  }

  function requestCloseSheet() {
    if (!state.sheetOpen) return;
    if (history.state && history.state.mygo === 'sheet') {
      history.back();
      return;
    }
    closeSheetUI();
  }

  function closeSheetWithAction(action) {
    if (!state.sheetOpen) {
      action();
      return;
    }
    if (history.state && history.state.mygo === 'sheet') {
      state.pendingSheetAction = action;
      history.back();
      return;
    }
    closeSheetUI();
    action();
  }

  function pushHistory(kind) {
    try { history.pushState({ mygo: kind }, ''); } catch (_error) { /* non-browser harness */ }
  }

  function handlePopState() {
    if (state.sheetOpen) {
      closeSheetUI();
      var action = state.pendingSheetAction;
      state.pendingSheetAction = null;
      if (action) action();
      return;
    }
    if (player.open) closePlayerUI();
  }

  function openSeries(series, options) {
    if (!state.source || !series || !series.id) return;
    options = options || {};
    if (state.sheetOpen) closeSheetUI();
    if (player.open) closePlayerUI();

    player.open = true;
    player.sourceId = state.source.id;
    player.sourceName = state.source.name;
    player.series = series;
    player.episodes = [];
    player.index = -1;
    player.quality = '';
    player.mediaError = '';
    player.mediaLoading = false;
    player.detailError = '';
    player.resumePosition = 0;
    player.resumeHint = {
      episodeId: options.episodeId || '',
      position: asPosition(options.position)
    };
    player.favorite = false;
    player.returnFocus = document.activeElement;
    player.favoriteBusy = false;
    player.lastSaveAt = 0;

    var library = currentLibrary();
    if (library) player.favorite = !!library.favorites[series.id];

    dom.player.hidden = false;
    document.body.classList.add('player-open');
    dom.app.setAttribute('aria-hidden', 'true');
    dom.playerDetailLoading.hidden = true;
    dom.playerDetailError.hidden = true;
    dom.mediaPlaceholder.hidden = true;
    dom.mediaErrorActions.hidden = true;
    dom.qualityBadge.hidden = true;
    clearMediaElement();
    renderPlayerMeta();
    updateFavoriteButton();
    updateTransport();

    if (!(history.state && history.state.mygo === 'player')) pushHistory('player');
    dom.playerBack.focus({ preventScroll: true });
    loadDetail();
  }

  async function loadDetail() {
    if (!player.open || !player.series) return;
    var epoch = ++player.detailEpoch;
    if (player.detailAbort) player.detailAbort.abort();
    var controller = new AbortController();
    player.detailAbort = controller;
    player.detailLoading = true;
    player.detailError = '';
    dom.playerDetailLoading.hidden = false;
    dom.playerDetailError.hidden = true;
    dom.mediaPlaceholder.hidden = true;
    renderPlayerMeta();
    updateTransport();

    try {
      var payload = await api.call(player.sourceId, 'Detail', { ID: player.series.id }, controller.signal);
      if (epoch !== player.detailEpoch || !player.open) return;
      var response = payload && typeof payload === 'object' ? payload : {};
      var series = normalizeSeries(response.Series, player.series.id) || player.series;
      var rawEpisodes = Array.isArray(response.Episodes) ? response.Episodes
        : (Array.isArray(response.episodes) ? response.episodes : []);
      var episodes = rawEpisodes.map(normalizeEpisode).filter(Boolean);
      if (!episodes.length) {
        throw new api.ApiError('该剧集暂无可播放剧集。', 0);
      }

      player.series = series;
      player.episodes = episodes;
      player.detailLoading = false;
      player.detailError = '';
      dom.playerDetailLoading.hidden = true;

      var hint = player.resumeHint || {};
      var historyEntry = state.library && state.library.history ? state.library.history[series.id] : null;
      var targetId = hint.episodeId || (historyEntry ? historyEntry.episode.id : '');
      var targetIndex = -1;
      if (targetId) targetIndex = episodes.findIndex(function (episode) { return episode.id === targetId; });
      if (targetIndex < 0 && historyEntry && historyEntry.episode.number) {
        targetIndex = episodes.findIndex(function (episode) { return episode.number === historyEntry.episode.number; });
      }
      if (targetIndex < 0) targetIndex = 0;

      var resume = 0;
      if (hint.episodeId && hint.position > 0 && episodes[targetIndex] && episodes[targetIndex].id === hint.episodeId) {
        resume = hint.position;
      } else if (historyEntry && episodes[targetIndex] && historyEntry.episode.id === episodes[targetIndex].id) {
        resume = historyEntry.position;
      }

      player.index = targetIndex;
      player.resumePosition = resume;
      player.resumeHint = null;
      renderPlayerMeta();
      updateFavoriteButton();
      updateTransport();
      startPlayback();
    } catch (error) {
      if (error && error.name === 'AbortError') return;
      if (epoch !== player.detailEpoch || !player.open) return;
      player.detailLoading = false;
      player.detailError = errorText(error, '剧集读取失败，请重试。');
      dom.playerDetailLoading.hidden = true;
      dom.playerDetailError.hidden = false;
      dom.playerDetailErrorText.textContent = player.detailError;
      updateTransport();
    } finally {
      if (epoch === player.detailEpoch) player.detailAbort = null;
    }
  }

  function renderPlayerMeta() {
    if (!player.series) return;
    dom.playerSource.textContent = player.sourceName || '来源';
    var episode = player.episodes[player.index];
    dom.playerEpisodeTag.textContent = episode ? episodeLabel(episode, player.index) : '准备中';
    dom.playerEyebrow.textContent = 'MYGO · ANDROID';
    dom.playerTitle.textContent = player.series.title;
    dom.playerEpisodeTitle.textContent = episode ? episodeTitle(episode, player.index) : '正在读取剧集';
    var description = player.series.description || player.series.release_note || '';
    dom.playerDescription.textContent = description;
    dom.playerDescription.hidden = !description;
    dom.qualityBadge.textContent = player.quality || '';
    dom.qualityBadge.hidden = !player.quality;
  }

  function updateFavoriteButton() {
    if (!dom.playerFavorite) return;
    dom.playerFavorite.classList.toggle('is-favorite', player.favorite);
    dom.playerFavorite.setAttribute('aria-pressed', player.favorite ? 'true' : 'false');
    dom.playerFavorite.setAttribute('aria-label', player.favorite ? '取消收藏' : '收藏');
    dom.playerFavorite.disabled = player.favoriteBusy;
  }

  function updateTransport() {
    var hasEpisodes = player.episodes.length > 0;
    var episode = hasEpisodes ? player.episodes[player.index] : null;
    dom.prevEpisode.disabled = !hasEpisodes || player.index <= 0;
    dom.nextEpisode.disabled = !hasEpisodes || player.index >= player.episodes.length - 1;
    dom.playPause.disabled = !episode;
    dom.openEpisodes.disabled = !hasEpisodes;
    dom.episodeOpenLabel.textContent = hasEpisodes
      ? '选集 · ' + episodeLabel(episode, player.index) + ' / 共 ' + player.episodes.length + ' 集'
      : '选集';
    updatePlayPauseIcon();
  }

  function updatePlayPauseIcon() {
    if (!dom.playPause) return;
    var paused = dom.video.paused;
    setIcon(dom.playPause, paused ? 'play' : 'pause');
    dom.playPause.setAttribute('aria-label', paused ? '播放' : '暂停');
  }

  function showMediaLoading(message) {
    player.mediaLoading = true;
    player.mediaError = '';
    dom.mediaPlaceholder.hidden = false;
    dom.mediaPlaceholder.querySelector('.spinner').hidden = false;
    dom.mediaMessage.textContent = message || '正在准备播放';
    dom.mediaErrorActions.hidden = true;
  }

  function hideMediaPlaceholder() {
    if (player.mediaError) return;
    player.mediaLoading = false;
    dom.mediaPlaceholder.hidden = true;
  }

  function showMediaError(message) {
    player.mediaLoading = false;
    player.mediaError = message;
    dom.mediaPlaceholder.hidden = false;
    dom.mediaPlaceholder.querySelector('.spinner').hidden = true;
    dom.mediaMessage.textContent = message;
    dom.mediaErrorActions.hidden = false;
    dropSession();
  }

  function clearMediaElement() {
    player.suppressMediaError = true;
    if (player.hls) {
      try { player.hls.destroy(); } catch (_error) { /* ignore */ }
      player.hls = null;
    }
    try {
      dom.video.pause();
      dom.video.removeAttribute('src');
      dom.video.load();
    } catch (_error) { /* ignore */ }
    player.autoPlayPending = false;
    player.pendingSeek = 0;
    player.suppressMediaError = false;
    updatePlayPauseIcon();
  }

  function releaseSession(session, sourceId) {
    if (!session || !sourceId) return;
    api.release(sourceId, session).catch(function () { /* sessions expire server-side */ });
  }

  function dropSession() {
    var session = player.session;
    var sourceId = player.sourceId;
    player.session = '';
    releaseSession(session, sourceId);
  }

  async function startPlayback() {
    if (!player.open || !player.series || !player.episodes.length) return;
    var episode = player.episodes[player.index];
    if (!episode) return;
    var sourceId = player.sourceId;
    var epoch = ++player.mediaEpoch;
    if (player.mediaAbort) player.mediaAbort.abort();
    var controller = new AbortController();
    player.mediaAbort = controller;

    var oldSession = player.session;
    player.session = '';
    clearMediaElement();
    releaseSession(oldSession, sourceId);
    player.quality = '';
    player.mediaError = '';
    player.resumePosition = player.resumePosition || 0;
    dom.qualityBadge.hidden = true;
    showMediaLoading('正在解析播放地址…');
    updateTransport();

    try {
      var payload = await api.call(sourceId, 'Resolve', { ID: episode.id, Quality: '' }, controller.signal);
      if (epoch !== player.mediaEpoch || !player.open) {
        var staleSession = payload && typeof payload === 'object' ? asString(payload.SessionID) : '';
        releaseSession(staleSession, sourceId);
        return;
      }
      if (!payload || typeof payload !== 'object') throw new api.ApiError('来源没有返回播放地址。', 0);
      var url = safeHttpUrl(payload.URL);
      var session = asString(payload.SessionID);
      if (!url) {
        releaseSession(session, sourceId);
        throw new api.ApiError('来源返回的播放地址不是有效的 HTTP 地址。', 0);
      }
      player.session = session;
      player.quality = asString(payload.Definition);
      dom.qualityBadge.textContent = player.quality;
      dom.qualityBadge.hidden = !player.quality;
      player.pendingSeek = player.resumePosition || player.pendingSeek;
      player.autoPlayPending = true;
      attachMedia(url, epoch);
    } catch (error) {
      if (error && error.name === 'AbortError') return;
      if (epoch !== player.mediaEpoch || !player.open) return;
      showMediaError(errorText(error, '播放地址解析失败，请重试。'));
    } finally {
      if (epoch === player.mediaEpoch) player.mediaAbort = null;
    }
  }

  function attachMedia(url, epoch) {
    if (epoch !== player.mediaEpoch) return;
    var hlsType = 'application/vnd.apple.mpegurl';
    var hlsUrl = isHlsUrl(url);
    if (hlsUrl) {
      if (dom.video.canPlayType(hlsType)) {
        dom.video.src = url;
        dom.video.load();
        showMediaLoading('正在加载视频…');
        return;
      }
      if (window.Hls && window.Hls.isSupported()) {
        var hls = new window.Hls({
          enableWorker: true,
          lowLatencyMode: false,
          backBufferLength: 60
        });
        player.hls = hls;
        hls.on(window.Hls.Events.ERROR, function (_event, data) {
          if (!data || !data.fatal || epoch !== player.mediaEpoch) return;
          var message = data.type === window.Hls.ErrorTypes.NETWORK_ERROR
            ? '视频网络加载失败，请检查本机媒体服务后重试。'
            : '视频无法解码，请重试或换一集。';
          if (player.hls) {
            try { player.hls.destroy(); } catch (_error) { /* ignore */ }
            player.hls = null;
          }
          showMediaError(message);
        });
        hls.on(window.Hls.Events.MANIFEST_PARSED, function () {
          if (epoch === player.mediaEpoch) hideMediaPlaceholder();
        });
        hls.loadSource(url);
        hls.attachMedia(dom.video);
        showMediaLoading('正在加载视频…');
        return;
      }
      showMediaError('当前设备不支持 HLS 播放，且本地播放组件未加载。');
      return;
    }

    dom.video.src = url;
    dom.video.load();
    showMediaLoading('正在加载视频…');
  }

  function attemptPlay() {
    var promise = dom.video.play();
    if (promise && typeof promise.catch === 'function') {
      promise.catch(function (error) {
        if (error && error.name === 'NotAllowedError') {
          showToast('浏览器阻止了自动播放，点击画面开始播放。');
        }
      });
    }
  }

  function togglePlayback() {
    if (!player.open) return;
    if (player.mediaError) {
      startPlayback();
      return;
    }
    if (!dom.video.getAttribute('src')) {
      if (player.episodes.length) showToast('视频正在准备，请稍候。');
      return;
    }
    if (dom.video.paused) {
      if (dom.video.ended) dom.video.currentTime = 0;
      attemptPlay();
    } else {
      dom.video.pause();
    }
  }

  function showPauseFlash(paused) {
    if (!player.open) return;
    setIcon(dom.pauseFlash.querySelector('svg'), paused ? 'play' : 'pause');
    dom.pauseFlash.hidden = false;
    window.clearTimeout(player.pauseTimer);
    player.pauseTimer = window.setTimeout(function () { dom.pauseFlash.hidden = true; }, 520);
  }

  function switchEpisode(index) {
    if (!player.open || index < 0 || index >= player.episodes.length) return;
    if (index === player.index) {
      if (dom.video.paused) attemptPlay();
      return;
    }
    flushProgress(true, true);
    player.index = index;
    player.resumePosition = 0;
    player.pendingSeek = 0;
    player.quality = '';
    player.mediaError = '';
    renderPlayerMeta();
    updateFavoriteButton();
    updateTransport();
    startPlayback();
  }

  function goRelative(delta) {
    if (!player.open || !player.episodes.length) return;
    var next = player.index + delta;
    if (next < 0) {
      showToast('已经是第一集。');
      return;
    }
    if (next >= player.episodes.length) {
      showToast('已经是最后一集。');
      return;
    }
    switchEpisode(next);
  }

  function saveProgress(position, keepalive) {
    if (!player.open || !player.series || !player.episodes[player.index]) return;
    if (!Number.isFinite(position) || position <= 0.2) return;
    var episode = player.episodes[player.index];
    var series = player.series;
    var sourceId = player.sourceId;
    var savedAt = new Date().toISOString();
    var library = currentLibrary();
    if (library && state.source && state.source.id === sourceId) {
      library.history[series.id] = {
        series: series,
        episode: episode,
        position: position,
        updated_at: savedAt
      };
    }
    player.lastSaveAt = Date.now();
    api.progress(sourceId, series, episode, position, { keepalive: !!keepalive }).catch(function () {
      /* progress is best effort; next flush retries */
    });
  }

  function flushProgress(force, keepalive) {
    if (!player.open || !player.episodes[player.index]) return;
    var position = dom.video.currentTime;
    if (!Number.isFinite(position)) position = 0;
    if (!force && Date.now() - player.lastSaveAt < SAVE_INTERVAL) return;
    if (position <= 0.2 && player.lastSaveAt && !force) return;
    saveProgress(position, keepalive);
  }

  async function toggleFavorite() {
    if (!player.open || !player.series || player.favoriteBusy) return;
    var sourceId = player.sourceId;
    var series = player.series;
    var desired = !player.favorite;
    player.favoriteBusy = true;
    updateFavoriteButton();
    try {
      var payload = await api.favorite(sourceId, series);
      if (!player.open || player.sourceId !== sourceId || !player.series || player.series.id !== series.id) return;
      var value = payload && payload.favorite === true;
      player.favorite = value;
      if (state.library && state.source && state.source.id === sourceId) {
        if (value) state.library.favorites[series.id] = series;
        else delete state.library.favorites[series.id];
      }
      showToast(value ? '已加入追剧收藏。' : '已取消收藏。');
    } catch (error) {
      if (player.open) showToast(errorText(error, '收藏操作失败，请重试。'), 3000);
    } finally {
      player.favoriteBusy = false;
      updateFavoriteButton();
    }
  }

  function openEpisodeSheet() {
    if (!player.open || !player.episodes.length) return;
    var currentRange = Math.floor(player.index / RANGE_SIZE);
    var range = currentRange;
    var ranges = Math.ceil(player.episodes.length / RANGE_SIZE);

    openSheet('选集', 'EPISODES', function (body) {
      body.appendChild(el('p', 'sheet-sub', player.series.title + ' · 共 ' + player.episodes.length + ' 集'));
      var tabs = el('div', 'range-tabs');
      for (var i = 0; i < ranges; i++) {
        (function (rangeIndex) {
          var start = rangeIndex * RANGE_SIZE + 1;
          var end = Math.min((rangeIndex + 1) * RANGE_SIZE, player.episodes.length);
          var tab = el('button', 'range-tab' + (rangeIndex === range ? ' is-active' : ''), start + '–' + end);
          tab.type = 'button';
          tab.setAttribute('aria-pressed', rangeIndex === range ? 'true' : 'false');
          tab.addEventListener('click', function () {
            range = rangeIndex;
            body.scrollTop = 0;
            draw();
          });
          tabs.appendChild(tab);
        })(i);
      }
      body.appendChild(tabs);
      var grid = el('div', 'episode-grid');
      body.appendChild(grid);

      function draw() {
        Array.prototype.forEach.call(tabs.children, function (tab, index) {
          tab.classList.toggle('is-active', index === range);
          tab.setAttribute('aria-pressed', index === range ? 'true' : 'false');
        });
        clear(grid);
        var start = range * RANGE_SIZE;
        var end = Math.min(start + RANGE_SIZE, player.episodes.length);
        for (var index = start; index < end; index++) {
          (function (episodeIndex) {
            var episode = player.episodes[episodeIndex];
            var number = episodeNumber(episode, episodeIndex);
            var cell = el('button', 'episode-cell' + (episodeIndex === player.index ? ' is-current' : ''), number);
            cell.type = 'button';
            cell.setAttribute('aria-label', episodeTitle(episode, episodeIndex));
            if (episodeIndex === player.index) cell.setAttribute('aria-current', 'true');
            cell.addEventListener('click', function () {
              closeSheetWithAction(function () { switchEpisode(episodeIndex); });
            });
            grid.appendChild(cell);
          })(index);
        }
        if (range === currentRange) {
          window.requestAnimationFrame(function () {
            var current = grid.querySelector('.episode-cell.is-current');
            if (!current) return;
            var cellRect = current.getBoundingClientRect();
            var bodyRect = body.getBoundingClientRect();
            if (cellRect.top < bodyRect.top + 54 || cellRect.bottom > bodyRect.bottom - 8) {
              body.scrollTop += cellRect.top - bodyRect.top - body.clientHeight / 2 + cellRect.height / 2;
            }
          });
        }
      }
      draw();
    });
  }

  function closePlayerUI() {
    if (!player.open) return;
    flushProgress(true, true);
    player.closing = true;
    var session = player.session;
    var sourceId = player.sourceId;
    player.session = '';
    player.open = false;
    player.mediaEpoch++;
    if (player.mediaAbort) {
      player.mediaAbort.abort();
      player.mediaAbort = null;
    }
    if (player.detailAbort) {
      player.detailAbort.abort();
      player.detailAbort = null;
    }
    clearMediaElement();
    releaseSession(session, sourceId);
    dom.player.hidden = true;
    dom.playerDetailLoading.hidden = true;
    dom.playerDetailError.hidden = true;
    dom.mediaPlaceholder.hidden = true;
    dom.player.removeAttribute('aria-hidden');
    document.body.classList.remove('player-open');
    dom.app.removeAttribute('aria-hidden');
    var focusTarget = player.returnFocus;
    player.returnFocus = null;
    player.closing = false;
    if (focusTarget && focusTarget.isConnected) {
      try { focusTarget.focus({ preventScroll: true }); } catch (_error) { /* ignore */ }
    }
  }

  function requestClosePlayer() {
    if (!player.open) return;
    if (state.sheetOpen) {
      requestCloseSheet();
      return;
    }
    if (history.state && history.state.mygo === 'player') {
      history.back();
      return;
    }
    closePlayerUI();
  }

  function activateSource(source, announce) {
    if (!source || !source.available) return;
    if (state.feedAbort) state.feedAbort.abort();
    if (state.libraryAbort) state.libraryAbort.abort();
    if (player.open) {
      var hadPlayerHistory = history.state && history.state.mygo === 'player';
      closePlayerUI();
      if (hadPlayerHistory) history.back();
    }
    if (state.sheetOpen) closeSheetUI();
    state.epoch++;
    state.source = source;
    state.nav = source.navigation.length ? source.navigation[0].value : '';
    state.navLabel = source.navigation.length ? source.navigation[0].label : '';
    state.sort = '';
    state.filterLabel = '';
    state.query = '';
    state.searchInput = '';
    state.items = [];
    state.cursor = '';
    state.hasMore = false;
    state.feedError = '';
    state.pageError = '';
    state.feedLoading = false;
    state.loadingMore = false;
    state.library = null;
    state.libraryError = '';
    state.libraryLoading = false;
    state.tab = 'discover';
    state.sourcesError = '';
    sourceStorageSet(source.id);
    syncTabbar();
    renderDiscover();
    dom.mainScroll.scrollTop = 0;
    loadFeed(false);
    loadLibrary(false);
    if (announce) showToast('已切换到 ' + source.name);
  }

  async function fetchSources(sequence) {
    state.sourcesLoaded = false;
    state.sourcesError = '';
    renderDiscover();
    var controller = new AbortController();
    state.bootAbort = controller;
    try {
      var payload = await api.sources(controller.signal);
      if (sequence !== state.bootSeq) return;
      var list = payload && Array.isArray(payload.sources) ? payload.sources : [];
      state.sources = list.map(normalizeSource).filter(Boolean);
      state.runtime = asString(payload && payload.runtime);
      state.backendVersion = asString(payload && payload.version);
      state.sourcesLoaded = true;
      var available = state.sources.filter(function (source) { return source.available; });
      if (!available.length) {
        state.source = null;
        renderDiscover();
        return;
      }
      var saved = sourceStorageGet();
      var selected = available.find(function (source) { return source.id === saved; }) || available[0];
      activateSource(selected, false);
    } catch (error) {
      if (error && error.name === 'AbortError') return;
      if (sequence !== state.bootSeq) return;
      state.sourcesLoaded = true;
      state.sources = [];
      state.source = null;
      state.sourcesError = errorText(error, '无法读取来源列表。');
      if (error && (error.status === 401 || error.status === 403)) {
        renderFatal(state.sourcesError);
      } else {
        renderDiscover();
      }
    } finally {
      if (sequence === state.bootSeq) state.bootAbort = null;
    }
  }

  async function boot() {
    var sequence = ++state.bootSeq;
    state.epoch++;
    if (state.bootAbort) state.bootAbort.abort();
    if (state.feedAbort) state.feedAbort.abort();
    if (state.libraryAbort) state.libraryAbort.abort();
    if (player.open) closePlayerUI();
    if (state.sheetOpen) closeSheetUI();
    state.tab = 'discover';
    state.source = null;
    state.sources = [];
    state.sourcesLoaded = false;
    state.sourcesError = '';
    state.items = [];
    state.library = null;
    dom.tabbar.hidden = false;
    dom.mainScroll.scrollTop = 0;
    syncTabbar();
    renderBoot();
    try {
      await api.preparePlayback();
      if (sequence !== state.bootSeq) return;
      await fetchSources(sequence);
    } catch (error) {
      if (sequence !== state.bootSeq) return;
      renderFatal(errorText(error, '应用启动失败。'));
    }
  }

  function renderBoot() {
    var nodes = [buildPageHead()];
    var intro = el('section', 'discover-intro');
    var copy = el('div');
    append(copy, el('p', 'eyebrow', 'MYGO · ANDROID'), el('h1', '', '正在准备'), el('p', '', '连接本机来源与播放环境。'));
    var mark = el('div', 'intro-mark');
    mark.appendChild(icon('discover'));
    append(intro, copy, mark);
    nodes.push(intro);
    nodes.push(stateBlock({ spinner: true, title: '正在连接', message: '正在读取应用令牌与内嵌来源。' }));
    setView(nodes);
  }

  function renderFatal(message) {
    dom.tabbar.hidden = true;
    dom.mainScroll.scrollTop = 0;
    var wrap = el('div', 'fatal-wrap');
    var brand = el('div', 'wordmark fatal-brand');
    brand.appendChild(document.createTextNode('MyGo'));
    brand.appendChild(el('i'));
    wrap.appendChild(brand);
    wrap.appendChild(stateBlock({
      icon: 'info',
      title: '无法启动移动播放器',
      message: message,
      actionLabel: '重新连接',
      onAction: boot
    }));
    wrap.appendChild(el('p', 'build-line', '需要从 MyGo 安卓应用内打开；普通浏览器不包含本机来源与播放桥接。'));
    setView(wrap);
  }

  function bindPlayerEvents() {
    dom.playerBack.addEventListener('click', requestClosePlayer);
    dom.playerFavorite.addEventListener('click', toggleFavorite);
    dom.playPause.addEventListener('click', togglePlayback);
    dom.prevEpisode.addEventListener('click', function () { goRelative(-1); });
    dom.nextEpisode.addEventListener('click', function () { goRelative(1); });
    dom.openEpisodes.addEventListener('click', openEpisodeSheet);
    dom.mediaRetry.addEventListener('click', startPlayback);
    dom.playerDetailRetry.addEventListener('click', loadDetail);

    dom.gestureZone.addEventListener('pointerdown', function (event) {
      if (event.pointerType === 'mouse' && event.button !== 0) return;
      player.pointer = { x: event.clientX, y: event.clientY, time: Date.now() };
    });
    function endPointer(event) {
      if (!player.pointer) return;
      var dx = event.clientX - player.pointer.x;
      var dy = event.clientY - player.pointer.y;
      var elapsed = Date.now() - player.pointer.time;
      player.pointer = null;
      if (elapsed < 650 && Math.abs(dy) > 62 && Math.abs(dy) > Math.abs(dx) * 1.15) {
        player.swipedAt = Date.now();
        if (dy < 0) goRelative(1);
        else goRelative(-1);
      }
    }
    dom.gestureZone.addEventListener('pointerup', endPointer);
    dom.gestureZone.addEventListener('pointercancel', function () { player.pointer = null; });
    dom.gestureZone.addEventListener('click', function () {
      if (Date.now() - player.swipedAt < 450) return;
      togglePlayback();
      showPauseFlash(dom.video.paused);
    });

    dom.seek.addEventListener('pointerdown', function () { player.seeking = true; });
    dom.seek.addEventListener('input', function () {
      var duration = dom.video.duration;
      if (!Number.isFinite(duration) || duration <= 0) return;
      var target = Math.max(0, Math.min(parseFloat(dom.seek.value) || 0, duration));
      try { dom.video.currentTime = target; } catch (_error) { /* ignore */ }
      updateSeekUI();
    });
    dom.seek.addEventListener('change', function () {
      player.seeking = false;
      flushProgress(true, false);
    });

    dom.video.addEventListener('loadedmetadata', function () {
      if (!player.open) return;
      var duration = dom.video.duration;
      var seekTo = player.pendingSeek;
      player.pendingSeek = 0;
      player.resumePosition = 0;
      if (seekTo > 0 && Number.isFinite(duration) && duration > 0) {
        try { dom.video.currentTime = Math.min(seekTo, Math.max(0, duration - 0.25)); } catch (_error) { /* ignore */ }
      }
      updateSeekUI();
      if (player.autoPlayPending) {
        player.autoPlayPending = false;
        attemptPlay();
      }
    });
    dom.video.addEventListener('durationchange', updateSeekUI);
    dom.video.addEventListener('timeupdate', function () {
      if (!player.open) return;
      if (!player.seeking) updateSeekUI();
      if (!dom.video.paused && Date.now() - player.lastSaveAt >= SAVE_INTERVAL) {
        flushProgress(false, false);
      }
    });
    dom.video.addEventListener('canplay', function () {
      if (player.open && !player.mediaError) hideMediaPlaceholder();
    });
    dom.video.addEventListener('playing', function () {
      if (!player.open) return;
      hideMediaPlaceholder();
      updatePlayPauseIcon();
    });
    dom.video.addEventListener('play', updatePlayPauseIcon);
    dom.video.addEventListener('pause', function () {
      updatePlayPauseIcon();
      if (player.open && !player.closing) flushProgress(true, false);
    });
    dom.video.addEventListener('error', function () {
      if (!player.open || player.suppressMediaError || player.mediaLoading || !dom.video.getAttribute('src')) return;
      var code = dom.video.error ? dom.video.error.code : 0;
      showMediaError(code === 4 ? '视频格式无法播放，请重试或换一集。' : '视频加载失败，请检查本机媒体服务。');
    });
    dom.video.addEventListener('ended', function () {
      if (!player.open) return;
      flushProgress(true, true);
      if (player.index < player.episodes.length - 1) {
        showToast('正在播放下一集…', 1200);
        var oldIndex = player.index;
        window.clearTimeout(player.endTimer);
        player.endTimer = window.setTimeout(function () {
          if (player.open && player.index === oldIndex) switchEpisode(oldIndex + 1);
        }, 380);
      } else {
        showToast('已经是最后一集。');
      }
    });
    dom.video.addEventListener('seeked', updateSeekUI);
  }

  function updateSeekUI() {
    if (!dom.seek) return;
    var duration = dom.video.duration;
    var current = dom.video.currentTime;
    var valid = Number.isFinite(duration) && duration > 0;
    if (valid) {
      dom.seek.max = String(duration);
      dom.seek.value = String(Math.min(current, duration));
      dom.seek.disabled = false;
      dom.timeDuration.textContent = formatTime(duration);
    } else {
      dom.seek.max = '0';
      dom.seek.value = '0';
      dom.seek.disabled = true;
      dom.timeDuration.textContent = '0:00';
    }
    dom.timeCurrent.textContent = formatTime(current);
    var percent = valid ? Math.max(0, Math.min(100, current / duration * 100)) : 0;
    dom.seek.style.setProperty('--p', percent + '%');
  }

  function bindGlobalEvents() {
    Array.prototype.forEach.call(dom.tabbar.querySelectorAll('.tab'), function (button) {
      button.addEventListener('click', function () { switchTab(button.getAttribute('data-tab')); });
    });
    dom.sheetClose.addEventListener('click', requestCloseSheet);
    dom.overlay.addEventListener('click', function (event) {
      if (event.target === dom.overlay) requestCloseSheet();
    });
    window.addEventListener('popstate', handlePopState);
    document.addEventListener('keydown', function (event) {
      if (event.key === 'Escape') {
        if (state.sheetOpen) {
          event.preventDefault();
          requestCloseSheet();
          return;
        }
        if (player.open) {
          event.preventDefault();
          requestClosePlayer();
        }
        return;
      }
      if (!player.open || state.sheetOpen) return;
      if (event.target && /^(INPUT|TEXTAREA|SELECT)$/.test(event.target.tagName)) return;
      if (event.key === 'ArrowUp') { event.preventDefault(); goRelative(-1); }
      else if (event.key === 'ArrowDown') { event.preventDefault(); goRelative(1); }
      else if (event.key === ' ') { event.preventDefault(); togglePlayback(); }
    });
    document.addEventListener('visibilitychange', function () {
      if (!player.open) return;
      if (document.visibilityState === 'hidden') {
        if (!dom.video.paused) dom.video.pause();
        flushProgress(true, true);
      }
    });
    window.addEventListener('mygo:suspend', function () {
      if (!player.open) return;
      if (!dom.video.paused) dom.video.pause();
      flushProgress(true, true);
    });
    window.addEventListener('pagehide', function () {
      if (player.open) flushProgress(true, true);
    });
  }

  function init() {
    dom = {
      app: byId('app'),
      mainScroll: byId('mainScroll'),
      view: byId('view'),
      tabbar: byId('tabbar'),
      player: byId('player'),
      video: byId('video'),
      playerBack: byId('playerBack'),
      playerFavorite: byId('playerFavorite'),
      playerSource: byId('playerSource'),
      playerEpisodeTag: byId('playerEpisodeTag'),
      playerEyebrow: byId('playerEyebrow'),
      playerTitle: byId('playerTitle'),
      playerEpisodeTitle: byId('playerEpisodeTitle'),
      playerDescription: byId('playerDescription'),
      mediaPlaceholder: byId('mediaPlaceholder'),
      mediaMessage: byId('mediaMessage'),
      mediaErrorActions: byId('mediaErrorActions'),
      mediaRetry: byId('mediaRetry'),
      gestureZone: byId('gestureZone'),
      pauseFlash: byId('pauseFlash'),
      seek: byId('seek'),
      timeCurrent: byId('timeCurrent'),
      timeDuration: byId('timeDuration'),
      prevEpisode: byId('prevEpisode'),
      playPause: byId('playPause'),
      nextEpisode: byId('nextEpisode'),
      openEpisodes: byId('openEpisodes'),
      episodeOpenLabel: byId('episodeOpenLabel'),
      qualityBadge: byId('qualityBadge'),
      playerDetailLoading: byId('playerDetailLoading'),
      playerDetailError: byId('playerDetailError'),
      playerDetailErrorText: byId('playerDetailErrorText'),
      playerDetailRetry: byId('playerDetailRetry'),
      overlay: byId('overlay'),
      sheet: byId('sheet'),
      sheetTitle: byId('sheetTitle'),
      sheetKicker: byId('sheetKicker'),
      sheetBody: byId('sheetBody'),
      sheetClose: byId('sheetClose'),
      toast: byId('toast')
    };

    if (!api) {
      renderFatal('前端接口脚本没有加载，请重新打开应用。');
      return;
    }
    bindGlobalEvents();
    bindPlayerEvents();
    boot();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
