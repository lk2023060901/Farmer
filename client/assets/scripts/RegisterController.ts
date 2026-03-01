import { _decorator, Component, EditBox, Button, director } from 'cc';
const { ccclass, property } = _decorator;

const API_BASE = 'http://localhost:9080';

@ccclass('RegisterController')
export class RegisterController extends Component {

    @property(EditBox)
    phoneInput: EditBox = null!;

    @property(EditBox)
    passwordInput: EditBox = null!;

    @property(Button)
    signUpButton: Button = null!;

    @property(Button)
    backButton: Button = null!;

    start() {
        this.signUpButton.node.on(Button.EventType.CLICK, this._onRegister, this);
        this.backButton.node.on(Button.EventType.CLICK, this._onBack, this);
    }

    onDestroy() {
        if (this.signUpButton?.node) this.signUpButton.node.off(Button.EventType.CLICK, this._onRegister, this);
        if (this.backButton?.node) this.backButton.node.off(Button.EventType.CLICK, this._onBack, this);
    }

    private async _onRegister() {
        const phone    = this.phoneInput.string.trim();
        const password = this.passwordInput.string.trim();
        if (!phone || !password) { console.warn('手机号和密码不能为空'); return; }
        if (password.length < 6) { console.warn('密码不能少于6位'); return; }

        this.signUpButton.interactable = false;
        try {
            const resp = await fetch(`${API_BASE}/api/v1/auth/register`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ phone, password }),
            });
            const json = await resp.json();
            if (!resp.ok || json.code !== 0) { console.error('注册失败:', json.message); return; }

            localStorage.setItem('token', json.data.token);
            localStorage.setItem('userId', json.data.userId);
            localStorage.setItem('nickname', json.data.nickname);
            director.loadScene('world');
        } catch (e) {
            console.error('网络错误:', e);
        } finally {
            this.signUpButton.interactable = true;
        }
    }

    private _onBack() {
        director.loadScene('login');
    }
}
