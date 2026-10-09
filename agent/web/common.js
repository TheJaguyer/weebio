// Shared by the agent's pages: theme loading, remote-friendly spatial navigation, small API helper.

// Same origin as stremio-web, so the theme picked in Settings applies here too.
(function applyTheme() {
    let theme = 'saga';
    try { theme = localStorage.getItem('weebio.theme') || theme; } catch (_) {}
    document.documentElement.dataset.theme = theme;
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = `/weebio/themes/${encodeURIComponent(theme)}.css`;
    document.head.appendChild(link);
})();

// Product name from the shared branding (same brand.json the stremio-web build uses).
(async function applyBrand() {
    try {
        const brand = await (await fetch('/weebio/brand/brand.json')).json();
        const fill = () => {
            for (const el of document.querySelectorAll('[data-brand-name]')) el.textContent = brand.name;
            // Inline SVG (not <img>) so the mark takes the theme's colours, like in the app.
            for (const el of document.querySelectorAll('[data-brand-svg]')) {
                fetch(`/weebio/brand/${el.dataset.brandSvg}.svg`).then((r) => r.text()).then((svg) => { el.innerHTML = svg; });
            }
            document.title = document.title ? `${document.title} · ${brand.name}` : brand.name;
        };
        document.readyState === 'loading' ? document.addEventListener('DOMContentLoaded', fill) : fill();
    } catch (_) { /* branding unavailable: pages still work */ }
})();

const weebio = {
    async api(path, options = {}) {
        const res = await fetch(`/weebio/api/${path}`, {
            ...options,
            headers: { 'Content-Type': 'application/json', 'X-Weebio': '1', ...(options.headers || {}) },
        });
        const body = await res.json().catch(() => ({}));
        if (!res.ok) throw Object.assign(new Error(body.message || res.statusText), { code: body.error });
        return body;
    },

    sleep: (ms) => new Promise((resolve) => setTimeout(resolve, ms)),

    // Called on Escape / Backspace (TV remote "Back"); pages override it.
    onBack: null,
};

// Arrow keys move focus to the nearest visible button in that direction; Enter activates it.
// TV remotes (via HDMI-CEC) and gamepads (via the agent) both arrive here as plain key presses.
(function spatialNavigation() {
    const focusables = () => [...document.querySelectorAll('button:not([disabled])')]
        .filter((el) => el.offsetParent !== null);

    const centre = (el) => {
        const r = el.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, r };
    };

    function move(dx, dy) {
        const items = focusables();
        const current = document.activeElement;
        if (!items.includes(current)) {
            items[0]?.focus();
            return;
        }
        const from = centre(current);
        let best = null;
        let bestScore = Infinity;
        for (const el of items) {
            if (el === current) continue;
            const to = centre(el);
            const along = (to.x - from.x) * dx + (to.y - from.y) * dy;   // distance in the pressed direction
            const across = Math.abs((to.x - from.x) * dy) + Math.abs((to.y - from.y) * dx);
            if (along <= 1) continue;
            const score = along + across * 2.5;                          // prefer staying in line
            if (score < bestScore) { bestScore = score; best = el; }
        }
        if (best) {
            best.focus();
            best.scrollIntoView({ block: 'nearest' });
        }
    }

    document.addEventListener('keydown', (e) => {
        const dirs = { ArrowUp: [0, -1], ArrowDown: [0, 1], ArrowLeft: [-1, 0], ArrowRight: [1, 0] };
        if (dirs[e.key]) {
            e.preventDefault();
            move(...dirs[e.key]);
        } else if (e.key === 'Escape' || (e.key === 'Backspace' && !weebio.capturesBackspace)) {
            if (weebio.onBack) { e.preventDefault(); weebio.onBack(); }
        }
    });

    // Keep something focused so the first remote press always does something visible.
    document.addEventListener('focusout', () => setTimeout(() => {
        if (!focusables().includes(document.activeElement)) focusables()[0]?.focus();
    }, 0));
    weebio.focusFirst = () => focusables()[0]?.focus();
})();
