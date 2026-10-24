(function () {
    const $ = (s) => document.querySelector(s);

    function updateCount() {
        const all = document.querySelectorAll('#dup-groups .dup-group');
        const total = all.length;
        const shown = [...all].filter((g) => !g.hidden).length;
        const label = total === 1 ? ' group' : ' groups';
        $('#dup-count').textContent = shown === total ? total + label : shown + ' of ' + total + label;
        const empty = $('#dup-empty');
        empty.textContent = total === 0 ? 'No similar images found.' : 'No groups match the filter.';
        empty.hidden = shown !== 0;
    }

    // filter results: space separated, case-insensitively against paths with -negation support
    function applyFilter() {
        const terms = $('#dup-filter').value.toLowerCase().split(/\s+/).filter(Boolean);
        const include = terms.filter((t) => t[0] !== '-').map((t) => t);
        const exclude = terms.filter((t) => t[0] === '-' && t.length > 1).map((t) => t.slice(1));
        document.querySelectorAll('#dup-groups .dup-group').forEach((g) => {
            const paths = [...g.querySelectorAll('.dup-card')].map((c) => c._item.path.toLowerCase());
            const ok = include.every((t) => paths.some((p) => p.includes(t))) &&
                !exclude.some((t) => paths.some((p) => p.includes(t)));
            g.hidden = !ok;
        });
        updateCount();
    }

    // highlight matching part of path in groups
    function highlightGroup(groupEl) {
        const cards = [...groupEl.querySelectorAll('.dup-card')];
        const paths = cards.map((c) => c._item.path);
        let pre = 0, suf = 0;
        if (paths.length >= 2) {
            const min = Math.min(...paths.map((p) => p.length));
            while (pre < min && paths.every((p) => p[pre] === paths[0][pre])) pre++;
            while (pre + suf < min && paths.every((p) => p[p.length - 1 - suf] === paths[0][paths[0].length - 1 - suf])) suf++;
        }
        cards.forEach((c) => {
            const p = c._item.path;
            const el = c.querySelector('.dup-path');
            el.textContent = '';
            const parts = [[p.slice(0, pre), true], [p.slice(pre, p.length - suf), false], [p.slice(p.length - suf), true]];
            for (const [text, match] of parts) {
                if (!text) continue;
                const span = document.createElement('span');
                if (match) span.className = 'dup-match';
                span.textContent = text;
                el.append(span);
            }
        });
    }

    let msgTimer;
    function showMsg(msg, isErr) {
        const el = $('#dup-msg');
        el.textContent = msg;
        el.className = isErr ? 'dup-msg--err' : 'dup-msg--ok';
        clearTimeout(msgTimer);
        msgTimer = setTimeout(() => { el.textContent = ''; }, 8000);
    }
    const showErr = (msg) => showMsg(msg, true);

    async function post(url, body) {
        const res = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        });
        if (!res.ok) throw new Error((await res.text()).trim() || res.status);
        return res;
    }

    // Tracks images in a group to only show replace for pairs
    function syncGroup(groupEl) {
        const n = groupEl.querySelectorAll('.dup-card').length;
        if (n < 2) { groupEl.remove(); } else { groupEl.dataset.count = n; highlightGroup(groupEl); }
        updateCount();
    }

    async function del(item, card, groupEl) {
        if (!confirm('Permanently delete this file?\n\n' + item.path)) return;
        try {
            await post('/admin/duplicates/delete', { path: item.path });
            card.remove();
            syncGroup(groupEl);
        } catch (e) {
            showErr('Delete failed: ' + e.message);
        }
    }

    async function delAll(groupEl) {
        const items = [...groupEl.querySelectorAll('.dup-card')].map((c) => c._item);
        if (!confirm('Permanently delete ALL ' + items.length + ' files in this group?\n\n' +
            items.map((i) => i.path).join('\n') + '\n\nNo copy will be kept. This cannot be undone.')) return;
        try {
            await post('/admin/duplicates/delete-group', { paths: items.map((i) => i.path) });
            showMsg('Deleted ' + items.length + ' files');
            groupEl.remove();
            updateCount();
        } catch (e) {
            showErr('Delete all failed: ' + e.message);
        }
    }

    // replace files prompt
    async function replaceOther(item, card, groupEl) {
        const other = [...groupEl.querySelectorAll('.dup-card')].find((c) => c !== card);
        if (!other) return;
        const dst = other._item;
        if (!confirm(
            'Replace this file:\n\n' + dst.path + '  (' + dst.dims + ', ' + dst.size + ')\n\n' +
            'with this one:\n\n' + item.path + '  (' + item.dims + ', ' + item.size + ')\n\n' +
            'The better file is moved to the first location and the original is overwritten. ' +
            'This cannot be undone.')) return;
        try {
            const res = await post('/admin/duplicates/replace', { src: item.path, dst: dst.path });
            const { path } = await res.json();
            showMsg('Replaced: ' + path);
            groupEl.remove(); // only one file is left, so the group is resolved
            updateCount();
        } catch (e) {
            showErr('Replace failed: ' + e.message);
        }
    }

    function render(groups) {
        const root = $('#dup-groups');
        root.textContent = '';
        groups.forEach((g, i) => {
            const groupEl = document.createElement('section');
            groupEl.className = 'dup-group';
            groupEl.dataset.count = g.length;
            const h = document.createElement('h3');
            h.textContent = 'Group ' + (i + 1) + ' (' + g.length + ' images)';
            const all = document.createElement('button');
            all.type = 'button';
            all.className = 'dup-btn dup-btn--delall'; all.textContent = 'Delete all';
            all.title = 'Delete every file in this group';
            all.onclick = () => delAll(groupEl);
            h.append(all);
            groupEl.append(h);

            const row = document.createElement('div');
            row.className = 'dup-row';
            g.forEach((item) => {
                const card = document.createElement('div');
                card.className = 'dup-card' + (item.similarity === 'Original' ? ' dup-card--original' : '');
                card._item = item;

                const url = '/admin/duplicates/image?path=' + encodeURIComponent(item.path);
                const a = document.createElement('a');
                a.href = url; a.target = '_blank';
                const img = document.createElement('img');
                img.loading = 'lazy'; img.src = url; img.alt = '';
                a.append(img);

                const p = document.createElement('div');
                p.className = 'dup-path'; p.textContent = item.path;

                const meta = document.createElement('div');
                meta.className = 'dup-meta';
                for (const [cls, text] of [['dup-tag--sim', item.similarity], ['', item.dims], ['', item.size]]) {
                    const t = document.createElement('span');
                    t.className = 'dup-tag ' + cls; t.textContent = text;
                    meta.append(t);
                }

                const btn = document.createElement('button');
                btn.type = 'button';
                btn.className = 'dup-btn dup-btn--del'; btn.textContent = 'Delete';
                btn.onclick = () => del(item, card, groupEl);

                const rep = document.createElement('button');
                rep.type = 'button';
                rep.className = 'dup-btn dup-btn--replace'; rep.textContent = 'Replace other with this';
                rep.title = 'Move this file over the other one in the pair';
                rep.onclick = () => replaceOther(item, card, groupEl);

                card.append(a, p, meta, btn, rep);
                row.append(card);
            });
            groupEl.append(row);
            root.append(groupEl);
            highlightGroup(groupEl);
        });
        $('#dup-bar').hidden = false;
        applyFilter();
    }

    async function scan(e) {
        e.preventDefault();
        const dirs = [$('#dup-dir1').value, $('#dup-dir2').value].map((d) => d.trim()).filter(Boolean);
        const btn = $('#dup-scan');
        const status = $('#dup-status');
        btn.disabled = true;
        status.textContent = 'Scanning… this can take a while for large directories.';
        try {
            const res = await post('/admin/duplicates/scan', { dirs });
            const data = await res.json();
            render(data.groups || []);
            status.textContent = '';
        } catch (err) {
            status.textContent = '';
            $('#dup-bar').hidden = false;
            showErr('Scan failed: ' + err.message);
        } finally {
            btn.disabled = false;
        }
    }

    document.addEventListener('DOMContentLoaded', function () {
        $('#dup-form').addEventListener('submit', scan);
        $('#dup-filter').addEventListener('input', applyFilter);
        // Show the previous scan again after a reload (results are kept in memory only).
        fetch('/admin/duplicates/groups').then((r) => r.json()).then((data) => {
            if (data.dirs && data.dirs.length) {
                $('#dup-dir1').value = data.dirs[0];
                $('#dup-dir2').value = data.dirs[1] || '';
                render(data.groups || []);
            }
        }).catch(() => {});
    });
})();
