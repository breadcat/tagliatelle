function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Appends a hidden input to a form
function appendHidden(form, name, value) {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = name;
    input.value = value;
    input.dataset.generated = '1';
    form.appendChild(input);
}

// Fades out and removes elements matching selector
function autoHideSuccessMessages(selector = '.auto-hide-success', delay = 5000) {
    document.querySelectorAll(selector).forEach(div => {
        setTimeout(() => {
            div.style.transition = 'opacity 0.5s';
            div.style.opacity = '0';
            setTimeout(() => div.remove(), 500);
        }, delay);
    });
}
