// 页面内确认弹窗：替代被浏览器拦截的原生 confirm()
(function () {
    'use strict';

    let resolver = null;

    function ensureDialog() {
        let dialog = document.getElementById('app-confirm-dialog');
        if (dialog) return dialog;

        dialog = document.createElement('dialog');
        dialog.id = 'app-confirm-dialog';
        dialog.className = 'app-confirm-dialog';
        dialog.innerHTML = [
            '<form method="dialog" class="app-confirm-form">',
            '  <h3 class="app-confirm-title">确认操作</h3>',
            '  <p class="app-confirm-message"></p>',
            '  <div class="app-confirm-actions">',
            '    <button type="submit" value="cancel" class="btn-secondary">取消</button>',
            '    <button type="submit" value="confirm" class="btn-primary danger">确定</button>',
            '  </div>',
            '</form>',
        ].join('');
        document.body.appendChild(dialog);

        dialog.addEventListener('close', function () {
            const ok = dialog.returnValue === 'confirm';
            if (resolver) {
                const resolve = resolver;
                resolver = null;
                resolve(ok);
            }
        });
        // 点击遮罩层视为取消
        dialog.addEventListener('click', function (event) {
            if (event.target === dialog) dialog.close('cancel');
        });
        return dialog;
    }

    /**
     * 打开确认弹窗。
     * @param {string} message 提示文案
     * @param {{title?: string, confirmText?: string, cancelText?: string}} [options]
     * @returns {Promise<boolean>}
     */
    function confirmAction(message, options) {
        const dialog = ensureDialog();
        const opts = options || {};
        if (typeof dialog.showModal !== 'function') {
            // 极老浏览器退回原生实现
            return Promise.resolve(window.confirm(message));
        }
        if (resolver) {
            resolver(false);
            resolver = null;
        }
        dialog.querySelector('.app-confirm-title').textContent = opts.title || '确认操作';
        dialog.querySelector('.app-confirm-message').textContent = message;
        dialog.querySelector('button[value="confirm"]').textContent = opts.confirmText || '确定';
        dialog.querySelector('button[value="cancel"]').textContent = opts.cancelText || '取消';
        dialog.returnValue = 'cancel';
        dialog.showModal();
        return new Promise(function (resolve) {
            resolver = resolve;
        });
    }

    window.confirmAction = confirmAction;
})();
