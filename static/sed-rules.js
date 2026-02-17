// sed-rules.js - Manage sed rules in admin interface

let sedRules = [];

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    sedRules = window.initialSedRules || [];
    renderSedRules();
    setupSedRulesForm();
});

function renderSedRules() {
    const container = document.getElementById('sed-rules');
    if (!container) return;
    
    container.innerHTML = '';
    
    sedRules.forEach((rule, index) => {
        const ruleDiv = document.createElement('div');
        ruleDiv.style.cssText = 'border: 1px solid #ddd; padding: 15px; margin-bottom: 15px; border-radius: 5px; background-color: #f8f9fa;';
        
        ruleDiv.innerHTML = `
            <div style="display: flex; justify-content: space-between; align-items: start; margin-bottom: 10px;">
                <h4 style="margin: 0; color: #333;">Rule ${index + 1}</h4>
                <button onclick="removeSedRule(${index})" style="background-color: #dc3545; color: white; padding: 5px 10px; border: none; border-radius: 3px; font-size: 12px; cursor: pointer;">
                    Remove
                </button>
            </div>
            
            <div style="margin-bottom: 10px;">
                <label style="display: block; font-weight: bold; margin-bottom: 5px; font-size: 13px;">Name:</label>
                <input type="text" value="${escapeHtml(rule.name)}" 
                       onchange="updateSedRule(${index}, 'name', this.value)"
                       placeholder="e.g., Remove URL Parameters"
                       style="width: 100%; padding: 6px; font-size: 13px; border: 1px solid #ccc; border-radius: 3px;">
            </div>
            
            <div style="margin-bottom: 10px;">
                <label style="display: block; font-weight: bold; margin-bottom: 5px; font-size: 13px;">Description:</label>
                <input type="text" value="${escapeHtml(rule.description)}" 
                       onchange="updateSedRule(${index}, 'description', this.value)"
                       placeholder="e.g., Removes brandIds and productId from URLs"
                       style="width: 100%; padding: 6px; font-size: 13px; border: 1px solid #ccc; border-radius: 3px;">
            </div>
            
            <div style="margin-bottom: 0;">
                <label style="display: block; font-weight: bold; margin-bottom: 5px; font-size: 13px;">Sed Command:</label>
                <input type="text" value="${escapeHtml(rule.command)}" 
                       onchange="updateSedRule(${index}, 'command', this.value)"
                       placeholder="e.g., s?[?&]brandIds=[0-9]\\+&productId=[0-9]\\+??g"
                       style="width: 100%; padding: 6px; font-size: 13px; font-family: monospace; border: 1px solid #ccc; border-radius: 3px;">
                <small style="color: #666;">Sed command syntax (e.g., s/old/new/g)</small>
            </div>
        `;
        
        container.appendChild(ruleDiv);
    });
}

function addSedRule() {
    sedRules.push({
        name: '',
        description: '',
        command: ''
    });
    renderSedRules();
}

function removeSedRule(index) {
    if (confirm('Remove this sed rule?')) {
        sedRules.splice(index, 1);
        renderSedRules();
    }
}

function updateSedRule(index, field, value) {
    if (sedRules[index]) {
        sedRules[index][field] = value;
    }
}

function setupSedRulesForm() {
    const form = document.getElementById('sedrules-form');
    if (!form) return;
    
    form.addEventListener('submit', function(e) {
        // Validate rules
        for (let i = 0; i < sedRules.length; i++) {
            const rule = sedRules[i];
            if (!rule.name || !rule.command) {
                e.preventDefault();
                alert(`Rule ${i + 1} is incomplete. Please fill in Name and Sed Command.`);
                return;
            }
        }
        
        // Update hidden field with JSON
        document.getElementById('sed_rules_json').value = JSON.stringify(sedRules);
        
        // Let the form submit normally (don't prevent default)
    });
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
