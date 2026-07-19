let aliasGroups = window.initialAliasGroups || [];

function renderAliasGroups() {
    const container = document.getElementById('alias-groups');
    container.innerHTML = '';

    aliasGroups.forEach((group, gi) => {
        if (!group.aliases) group.aliases = [];

        const row = document.createElement('div');
        row.className = 'alias-row';
        row.dataset.group = gi;

        // Category input
        const catInput = document.createElement('input');
        catInput.type = 'text';
        catInput.className = 'alias-input alias-input--category';
        catInput.value = group.category;
        catInput.placeholder = 'category';
        catInput.addEventListener('change', e => { aliasGroups[gi].category = e.target.value; });
        row.appendChild(catInput);

        // Arrow separator
        const arrow = document.createElement('span');
        arrow.className = 'alias-separator';
        arrow.textContent = '→';
        row.appendChild(arrow);

        // Alias value inputs
        const valuesWrapper = document.createElement('span');
        valuesWrapper.id = `aliases-${gi}`;
        valuesWrapper.style.cssText = 'display:inline-flex; gap:4px; flex-wrap:wrap; align-items:center;';
        row.appendChild(valuesWrapper);
        renderAliasValues(gi, valuesWrapper);

        // + Value button
        const addBtn = document.createElement('button');
        addBtn.type = 'button';
        addBtn.className = 'alias-btn';
        addBtn.textContent = '+ Value';
        addBtn.addEventListener('click', () => addAlias(gi));
        row.appendChild(addBtn);

        // Remove group button
        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.className = 'alias-btn alias-btn--remove';
        removeBtn.textContent = '✕ Group';
        removeBtn.addEventListener('click', () => removeAliasGroup(gi));
        row.appendChild(removeBtn);

        container.appendChild(row);
    });
}

function renderAliasValues(gi, wrapper) {
    if (!wrapper) wrapper = document.getElementById(`aliases-${gi}`);
    wrapper.innerHTML = '';

    aliasGroups[gi].aliases.forEach((alias, ai) => {
        if (ai > 0) {
            const comma = document.createElement('span');
            comma.className = 'alias-separator';
            comma.textContent = ',';
            wrapper.appendChild(comma);
        }

        const input = document.createElement('input');
        input.type = 'text';
        input.className = 'alias-input alias-input--value';
        input.value = alias;
        input.placeholder = 'value';
        input.addEventListener('change', e => updateAlias(gi, ai, e.target.value));

        // Double-click to remove a single value
        input.title = 'Double-click to remove';
        input.addEventListener('dblclick', () => removeAlias(gi, ai));

        wrapper.appendChild(input);
    });
}

function addAliasGroup() {
    aliasGroups.push({ category: '', aliases: ['', ''] });
    renderAliasGroups();
}

function removeAliasGroup(gi) {
    if (confirm('Remove this alias group?')) {
        aliasGroups.splice(gi, 1);
        renderAliasGroups();
    }
}

function updateCategory(gi, value) {
    aliasGroups[gi].category = value;
}

function addAlias(gi) {
    aliasGroups[gi].aliases.push('');
    renderAliasValues(gi);
}

function removeAlias(gi, ai) {
    aliasGroups[gi].aliases.splice(ai, 1);
    renderAliasValues(gi);
}

function updateAlias(gi, ai, value) {
    aliasGroups[gi].aliases[ai] = value;
}

// submit form
document.getElementById('aliases-form').addEventListener('submit', function () {
    const cleanedGroups = aliasGroups
        .filter(g => g.category && g.aliases && g.aliases.length > 0)
        .map(g => ({
            category: g.category.trim(),
            aliases: g.aliases.filter(a => a && a.trim()).map(a => a.trim())
        }))
        .filter(g => g.aliases.length >= 2);

    this.querySelectorAll('input[data-generated]').forEach(el => el.remove());

    cleanedGroups.forEach((group, gi) => {
        appendHidden(this, `aliases[${gi}][category]`, group.category);
        group.aliases.forEach((alias, ai) => {
            appendHidden(this, `aliases[${gi}][aliases][${ai}]`, alias);
        });
    });
});

function appendHidden(form, name, value) {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = name;
    input.value = value;
    input.dataset.generated = '1';
    form.appendChild(input);
}

// initialise
document.addEventListener('DOMContentLoaded', renderAliasGroups);
