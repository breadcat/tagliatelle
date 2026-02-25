function activateTab(contentPrefix, btnSelector, tabPrefix, tabName) {
    document.querySelectorAll(btnSelector).forEach(btn => {
        btn.style.borderBottomColor = 'transparent';
        btn.style.fontWeight = 'normal';
    });

    document.querySelectorAll(`[id^="${contentPrefix}"]`).forEach(el => {
        el.style.display = 'none';
    });

    const selectedContent = document.getElementById(contentPrefix + tabName);
    if (selectedContent) selectedContent.style.display = 'block';

    const selectedBtn = document.getElementById(tabPrefix + tabName);
    if (selectedBtn) {
        selectedBtn.style.borderBottomColor = '#007bff';
        selectedBtn.style.fontWeight = 'bold';
    }
}

function showAdminTab(tabName) {
    activateTab('admin-content-', '.admin-tab-btn', 'admin-tab-', tabName);
}

function showThumbnailSubTab(subTabName) {
    activateTab('thumb-content-', '.thumb-subtab-btn', 'thumb-subtab-', subTabName);
}

document.addEventListener('DOMContentLoaded', function() {
    showAdminTab('settings');
    showThumbnailSubTab('missing');

    document.querySelectorAll('.auto-hide-success').forEach(div => {
        setTimeout(() => {
            div.style.transition = 'opacity 0.5s';
            div.style.opacity = '0';
            setTimeout(() => div.remove(), 500);
        }, 5000);
    });
});