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
// TV remotes (via HDMI-CEC) arrive here as plain key presses; gamepads are translated below.
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
        // Buttons in line with the current one (same row for left/right, overlapping columns for up/down)
        // win over anything diagonal, so e.g. Right from Cancel goes straight to Connect.
        const inLine = (r) => dx !== 0
            ? r.top < from.r.bottom - 1 && r.bottom > from.r.top + 1
            : r.left < from.r.right - 1 && r.right > from.r.left + 1;
        const ahead = items.filter((el) => {
            if (el === current) return false;
            const to = centre(el);
            return (to.x - from.x) * dx + (to.y - from.y) * dy > 1;
        });
        const lined = ahead.filter((el) => inLine(el.getBoundingClientRect()));
        let best = null;
        let bestScore = Infinity;
        for (const el of lined.length > 0 ? lined : ahead) {
            const to = centre(el);
            const along = (to.x - from.x) * dx + (to.y - from.y) * dy;   // distance in the pressed direction
            const across = Math.abs((to.x - from.x) * dy) + Math.abs((to.y - from.y) * dx);
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

// Gamepads (Gamepad API): D-pad / left stick move, A presses, B goes back, X deletes and Y types a
// space while a keyboard is on screen. Each is turned into the key press the page already handles.
(function gamepadNavigation() {
    if (typeof navigator.getGamepads !== 'function') return;
    const BUTTONS = { 0: 'Enter', 1: 'Escape', 2: 'Backspace', 3: ' ', 12: 'ArrowUp', 13: 'ArrowDown', 14: 'ArrowLeft', 15: 'ArrowRight' };
    const REPEAT_DELAY = 400;   // ms before a held direction starts repeating
    const REPEAT_EVERY = 120;
    const held = new Map();     // key -> time of the next repeat

    function press(key) {
        if (key === 'Enter') {
            document.activeElement?.click?.();
        } else if ((key === 'Backspace' || key === ' ') && !weebio.capturesBackspace) {
            // X / Y only edit text; never let X act as "back".
        } else {
            document.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
        }
    }

    function poll() {
        const now = performance.now();
        const down = new Set();
        for (const pad of navigator.getGamepads()) {
            if (!pad) continue;
            for (const [i, key] of Object.entries(BUTTONS)) if (pad.buttons[i]?.pressed) down.add(key);
            const [x = 0, y = 0] = pad.axes;
            if (x < -0.5) down.add('ArrowLeft');
            if (x > 0.5) down.add('ArrowRight');
            if (y < -0.5) down.add('ArrowUp');
            if (y > 0.5) down.add('ArrowDown');
            // Controllers without a standard mapping report the D-pad as a hat on axes 6/7.
            if (pad.mapping !== 'standard' && pad.axes.length >= 8) {
                if (pad.axes[6] < -0.5) down.add('ArrowLeft');
                if (pad.axes[6] > 0.5) down.add('ArrowRight');
                if (pad.axes[7] < -0.5) down.add('ArrowUp');
                if (pad.axes[7] > 0.5) down.add('ArrowDown');
            }
        }
        for (const key of down) {
            if (!held.has(key)) {
                held.set(key, now + REPEAT_DELAY);
                press(key);
            } else if (key.startsWith('Arrow') && now >= held.get(key)) {
                held.set(key, now + REPEAT_EVERY);
                press(key);
            }
        }
        for (const key of [...held.keys()]) if (!down.has(key)) held.delete(key);
        requestAnimationFrame(poll);
    }
    requestAnimationFrame(poll);
})();
