// 初始化向导页脚本（从 setup.html 外置，全站 CSP script-src 'self' 会拦截内联脚本）
(function() {
    var form = document.getElementById('setupForm');
    var errorArea = document.getElementById('errorArea');
    var errorText = document.getElementById('errorText');
    var submitBtn = document.getElementById('submitBtn');

    function showError(msg) {
        errorText.textContent = msg;
        errorArea.classList.add('visible');
    }

    function hideError() {
        errorArea.classList.remove('visible');
    }

    function setLoading(loading) {
        submitBtn.disabled = loading;
        if (loading) {
            submitBtn.innerHTML = '<span class="spinner"></span>正在初始化...';
        } else {
            submitBtn.textContent = '初始化系统';
        }
    }

    // 本位币选择卡片高亮联动
    var currencyOptions = document.querySelectorAll('.currency-option');
    function syncCurrencyHighlight() {
        var selected = document.querySelector('input[name="currency"]:checked');
        currencyOptions.forEach(function(opt) {
            opt.classList.toggle('selected', opt.querySelector('input').value === selected.value);
        });
    }
    currencyOptions.forEach(function(opt) {
        opt.querySelector('input').addEventListener('change', syncCurrencyHighlight);
    });
    syncCurrencyHighlight();

    form.addEventListener('submit', function(e) {
        e.preventDefault();
        hideError();

        var username = document.getElementById('username').value.trim();
        var displayName = document.getElementById('displayName').value.trim();
        var password = document.getElementById('password').value;
        var confirmPassword = document.getElementById('confirmPassword').value;
        var currency = document.querySelector('input[name="currency"]:checked').value;

        if (!username) {
            showError('请输入用户名');
            document.getElementById('username').focus();
            return;
        }
        if (!password) {
            showError('请输入密码');
            document.getElementById('password').focus();
            return;
        }
        if (password !== confirmPassword) {
            showError('两次输入的密码不一致');
            document.getElementById('confirmPassword').focus();
            return;
        }

        setLoading(true);

        fetch('/api/setup/initialize', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                username: username,
                displayName: displayName,
                password: password,
                confirmPassword: confirmPassword,
                currency: currency
            })
        })
        .then(function(res) { return res.json(); })
        .then(function(data) {
            if (data.code === 0) {
                window.location.href = '/admin/#/admin/login';
            } else {
                showError(data.message || '初始化失败，请重试');
                setLoading(false);
            }
        })
        .catch(function() {
            showError('网络错误，请检查连接后重试');
            setLoading(false);
        });
    });
})();
