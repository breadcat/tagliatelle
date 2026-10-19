let magicLinkRules = [];

// Init on page load
document.addEventListener('DOMContentLoaded', function() {
    magicLinkRules = window.initialMagicLinks || [];
    renderMagicLinkRules();
    setupMagicLinksForm();
});

function renderMagicLinkRules() {
    const container = document.getElementById('magic-link-rules');
    if (!container) return;

    container.innerHTML = '';

    magicLinkRules.forEach((rule, index) => {
        const ruleDiv = document.createElement('div');
        ruleDiv.style.cssText = 'border: 1px solid #ddd; padding: 15px; margin-bottom: 15px; border-radius: 5px;';

        ruleDiv.innerHTML = `
            <div style="display: flex; justify-content: space-between; align-items: start; margin-bottom: 10px;">
                <h4 style="margin: 0;">Rule ${index + 1}</h4>
                <button onclick="removeMagicLinkRule(${index})" style="background-color: #dc3545; color: white; padding: 5px 10px; border: none; border-radius: 3px; font-size: 12px; cursor: pointer;">
                    Remove
                </button>
            </div>

            <div style="margin-bottom: 10px;">
                <label style="display: block; font-weight: bold; margin-bottom: 5px; font-size: 13px;">Name:</label>
                <input type="text" value="${escapeHtml(rule.name)}"
                       onchange="updateMagicLinkRule(${index}, 'name', this.value)"
                       placeholder="e.g., Open on website.tld"
                       style="width: 100%; padding: 6px; font-size: 13px; border: 1px solid #ccc; border-radius: 3px;">
                <small>Shown as the label next to the link in the file viewer</small>
            </div>

            <div style="display: flex; gap: 10px; margin-bottom: 10px;">
                <div style="flex: 1;">
                    <label style="display: block; font-weight: bold; margin-bottom: 5px; font-size: 13px;">If tag category is:</label>
                    <input type="text" value="${escapeHtml(rule.category)}"
                           onchange="updateMagicLinkRule(${index}, 'category', this.value)"
                           placeholder="e.g., source"
                           style="width: 100%; padding: 6px; font-size: 13px; border: 1px solid #ccc; border-radius: 3px;">
                </div>
                <div style="flex: 1;">
                    <label style="display: block; font-weight: bold; margin-bottom: 5px; font-size: 13px;">equal to:</label>
                    <input type="text" value="${escapeHtml(rule.value)}"
                           onchange="updateMagicLinkRule(${index}, 'value', this.value)"
                           placeholder="e.g., example"
                           style="width: 100%; padding: 6px; font-size: 13px; border: 1px solid #ccc; border-radius: 3px;">
                </div>
            </div>

            <div style="margin-bottom: 0;">
                <label style="display: block; font-weight: bold; margin-bottom: 5px; font-size: 13px;">URL Template:</label>
                <input type="text" value="${escapeHtml(rule.template)}"
                       onchange="updateMagicLinkRule(${index}, 'template', this.value)"
                       placeholder="e.g., https://website.tld/$name"
                       style="width: 100%; padding: 6px; font-size: 13px; font-family: monospace; border: 1px solid #ccc; border-radius: 3px;">
                <small>Must start with http:// or https://. Use $category or \${category} to insert the file's tag value.</small>
            </div>
        `;

        container.appendChild(ruleDiv);
    });
}

function addMagicLinkRule() {
    magicLinkRules.push({
        name: '',
        category: '',
        value: '',
        template: ''
    });
    renderMagicLinkRules();
}

function removeMagicLinkRule(index) {
    if (confirm('Remove this magic link rule?')) {
        magicLinkRules.splice(index, 1);
        renderMagicLinkRules();
    }
}

function updateMagicLinkRule(index, field, value) {
    if (magicLinkRules[index]) {
        magicLinkRules[index][field] = value;
    }
}

function setupMagicLinksForm() {
    const form = document.getElementById('magiclinks-form');
    if (!form) return;

    form.addEventListener('submit', function(e) {
        // Validate rules
        for (let i = 0; i < magicLinkRules.length; i++) {
            const rule = magicLinkRules[i];
            if (!rule.name || !rule.category || !rule.value || !rule.template) {
                e.preventDefault();
                alert(`Rule ${i + 1} is incomplete. Please fill in Name, tag category, tag value and URL Template.`);
                return;
            }
            if (!/^https?:\/\//i.test(rule.template.trim())) {
                e.preventDefault();
                alert(`Rule ${i + 1}: URL Template must start with http:// or https://`);
                return;
            }
        }

        // Remove any previously-injected hidden fields from a prior submit
        this.querySelectorAll('input[data-generated]').forEach(el => el.remove());

        // Append one hidden field per value
        magicLinkRules.forEach((rule, i) => {
            appendHidden(this, `magic_links[${i}][name]`,     rule.name);
            appendHidden(this, `magic_links[${i}][category]`, rule.category);
            appendHidden(this, `magic_links[${i}][value]`,    rule.value);
            appendHidden(this, `magic_links[${i}][template]`, rule.template);
        });
    });
}
