(function () {

    const btn = document.getElementById('get-code');
    const statusElement = document.getElementById('code-status');

    btn.addEventListener('click', () => {
        btn.classList.add('disabled');
        statusElement.textContent = 'Sending...'
        fetch('/2fa/stage1').then(() => {
            btn.classList.remove('disabled');
            statusElement.classList.add('success');
            statusElement.textContent = 'Code sent. Check your email.';
        }).catch((err) => {
            btn.classList.remove('disabled');
            statusElement.classList.add('error');
            statusElement.textContent = '' + err;
        });
    });

    const errorMsg = new URL(window.location).searchParams.get('error');
    if (errorMsg) {
        statusElement.textContent = errorMsg;
        statusElement.classList.add('error');
    }

})();
