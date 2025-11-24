(function () {
  'use strict';

  const HX_INITIALIZED = 'data-hx-initialized';

  function toMilliseconds(value) {
    if (!value) {
      return 0;
    }
    const trimmed = value.trim();
    if (trimmed.endsWith('ms')) {
      return parseFloat(trimmed.slice(0, -2));
    }
    if (trimmed.endsWith('s')) {
      return parseFloat(trimmed.slice(0, -1)) * 1000;
    }
    const numeric = parseFloat(trimmed);
    return Number.isNaN(numeric) ? 0 : numeric;
  }

  function resolveTarget(element, targetSpec) {
    if (targetSpec === 'this') {
      return element;
    }
    if (typeof targetSpec === 'string' && targetSpec.length > 0) {
      return document.querySelector(targetSpec);
    }
    return element;
  }

  function resolveIndicator(element) {
    if (!element) {
      return null;
    }
    const selector = element.getAttribute('hx-indicator');
    let indicator = null;
    if (selector) {
      indicator = selector === 'this' ? element : document.querySelector(selector);
    }
    if (!indicator) {
      indicator = element.querySelector('.htmx-indicator');
    }
    if (indicator && !indicator.hasAttribute('data-hx-indicator')) {
      indicator.setAttribute('data-hx-indicator', 'true');
      indicator.style.display = 'none';
    }
    return indicator;
  }

  function setIndicatorVisibility(indicator, isVisible) {
    if (!indicator) {
      return;
    }
    indicator.style.display = isVisible ? '' : 'none';
  }

  function dispatchGlobalEvent(name, detail) {
    if (!document.body) {
      return;
    }
    document.body.dispatchEvent(new CustomEvent(name, { detail }));
  }

  function swapContent(target, swapStyle, html) {
    if (!target) {
      return;
    }
    const mode = (swapStyle || 'innerHTML').toLowerCase();
    if (mode === 'outerhtml') {
      target.outerHTML = html;
      scanForHx(document);
      return;
    }
    if (mode === 'beforeend') {
      target.insertAdjacentHTML('beforeend', html);
      scanForHx(target);
      return;
    }
    if (mode === 'afterend') {
      target.insertAdjacentHTML('afterend', html);
      scanForHx(document);
      return;
    }
    target.innerHTML = html;
    scanForHx(target);
  }

  function performFetch(contextEl, method, url, options = {}) {
    if (!url) {
      return Promise.resolve();
    }
    const target = resolveTarget(contextEl, options.target || (contextEl && contextEl.getAttribute('hx-target')));
    if (!target) {
      console.warn('htmx-lite: target not found for', url);
      return Promise.resolve();
    }
    const swap = options.swap || (contextEl && contextEl.getAttribute('hx-swap')) || 'innerHTML';
    const indicator = options.indicator || resolveIndicator(contextEl);
    const headers = Object.assign(
      {
        'X-Requested-With': 'XMLHttpRequest',
        Accept: 'text/html',
        'HX-Request': 'true',
      },
      options.headers || {}
    );
    const detail = {
      headers,
      method: method || 'GET',
      url,
      element: contextEl,
    };
    dispatchGlobalEvent('htmx:configRequest', detail);
    setIndicatorVisibility(indicator, true);
    dispatchGlobalEvent('htmx:beforeRequest', { element: contextEl, target, url });

    const controller = new AbortController();
    const timeoutMs = typeof options.timeout === 'number' ? options.timeout : 15000;
    let timeoutId;
    if (timeoutMs > 0) {
      timeoutId = setTimeout(() => {
        controller.abort();
        dispatchGlobalEvent('htmx:timeout', { element: contextEl, target, url });
      }, timeoutMs);
    }

    return fetch(url, {
      method: method || 'GET',
      headers,
      credentials: 'same-origin',
      signal: controller.signal,
    })
      .then((response) => {
        if (!response.ok) {
          dispatchGlobalEvent('htmx:responseError', {
            element: contextEl,
            target,
            url,
            status: response.status,
          });
          throw new Error(`Request failed: ${response.status}`);
        }
        return response.text();
      })
      .then((html) => {
        swapContent(target, swap, html);
        dispatchGlobalEvent('htmx:afterRequest', { element: contextEl, target, url });
      })
      .catch((error) => {
        if (error.name === 'AbortError') {
          dispatchGlobalEvent('htmx:sendAbort', { element: contextEl, target, url });
        } else {
          dispatchGlobalEvent('htmx:sendError', { element: contextEl, target, url, error });
          console.error('htmx-lite request failed', error);
        }
      })
      .finally(() => {
        if (timeoutId) {
          clearTimeout(timeoutId);
        }
        setIndicatorVisibility(indicator, false);
      });
  }

  function setupTriggers(element, handler, triggerAttr) {
    if (!triggerAttr) {
      element.addEventListener('click', handler);
      return;
    }
    const parts = triggerAttr
      .split(',')
      .map((part) => part.trim())
      .filter(Boolean);

    parts.forEach((part) => {
      if (part.startsWith('load')) {
        const delayMatch = part.match(/delay:([0-9.]+(?:ms|s)?)/i);
        const delay = delayMatch ? toMilliseconds(delayMatch[1]) : 0;
        setTimeout(() => handler(), delay);
      } else if (part.startsWith('every')) {
        const intervalMatch = part.match(/every\s+([0-9.]+(?:ms|s)?)/i);
        const interval = intervalMatch ? toMilliseconds(intervalMatch[1]) : 0;
        if (interval > 0) {
          setInterval(() => handler(), interval);
        }
      } else {
        element.addEventListener(part, handler);
      }
    });
  }

  function initElement(element) {
    if (!element || element.hasAttribute(HX_INITIALIZED)) {
      return;
    }
    const url = element.getAttribute('hx-get');
    if (!url) {
      return;
    }
    element.setAttribute(HX_INITIALIZED, 'true');

    const handler = (event) => {
      if (event) {
        event.preventDefault();
      }
      const currentUrl = element.getAttribute('hx-get');
      performFetch(element, 'GET', currentUrl);
    };
    element.__hxRequest = handler;

    const triggerAttr = element.getAttribute('hx-trigger');
    setupTriggers(element, handler, triggerAttr);
  }

  function scanForHx(root) {
    const scope =
      root && root.querySelectorAll
        ? root
        : document;

    if (scope instanceof Element && scope.matches('[hx-get]')) {
      initElement(scope);
    }

    scope.querySelectorAll('[hx-get]').forEach((el) => initElement(el));
  }

  function trigger(target, eventName) {
    const element =
      typeof target === 'string' ? document.querySelector(target) : target;
    if (!element) {
      return;
    }
    element.dispatchEvent(new CustomEvent(eventName, { bubbles: true }));
    if (eventName === 'load' && typeof element.__hxRequest === 'function') {
      element.__hxRequest();
    }
  }

  function ajax(method, url, options = {}) {
    return performFetch(
      null,
      method,
      url,
      {
        target: options.target,
        swap: options.swap,
        timeout: options.timeout,
        indicator: options.indicator ? document.querySelector(options.indicator) : null,
        headers: options.headers,
      }
    );
  }

  window.htmx = {
    trigger,
    ajax,
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => scanForHx(document));
  } else {
    scanForHx(document);
  }
})();
