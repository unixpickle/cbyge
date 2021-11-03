const btn = document.getElementById('get-code');
btn.addEventListener('click', () => {
    btn.style.pointerEvents = 'none';
    btn.style.opacity = '0.5';
    btn.textContent = 'Sending...';
    fetch('/2fa/stage1').then(() => {
        btn.textContent = 'Sent.';
    }).catch((err) => {
        btn.textContent = err;
    });
});
