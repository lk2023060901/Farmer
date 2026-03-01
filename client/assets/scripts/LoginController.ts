import { _decorator, Component, EditBox, Button, director } from 'cc';
const { ccclass, property } = _decorator;

const API_BASE = 'http://localhost:9080';

@ccclass('LoginController')
export class LoginController extends Component {

    @property(EditBox)
    phoneInput: EditBox = null!;

    @property(EditBox)
    passwordInput: EditBox = null!;

    @property(Button)
    signInButton: Button = null!;

    @property(Button)
    signUpButton: Button = null!;

    start() {
        this.signInButton.node.on(Button.EventType.CLICK, this._onSignIn, this);
        this.signUpButton.node.on(Button.EventType.CLICK, this._onSignUp, this);
    }

    onDestroy() {
        if (this.signInButton?.node) this.signInButton.node.off(Button.EventType.CLICK, this._onSignIn, this);
        if (this.signUpButton?.node) this.signUpButton.node.off(Button.EventType.CLICK, this._onSignUp, this);
    }

    private async _onSignIn() {
        const phone    = this.phoneInput.string.trim();
        const password = this.passwordInput.string.trim();
        if (!phone || !password) { console.warn('手机号和密码不能为空'); return; }

        this.signInButton.interactable = false;
        try {
            const resp = await fetch(`${API_BASE}/api/v1/auth/login`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ phone, password }),
            });
            const json = await resp.json();
            if (!resp.ok || json.code !== 0) { console.error('登录失败:', json.message); return; }

            localStorage.setItem('token', json.data.token);
            localStorage.setItem('userId', json.data.userId);
            localStorage.setItem('nickname', json.data.nickname);
            director.loadScene('world');
        } catch (e) {
            console.error('网络错误:', e);
        } finally {
            this.signInButton.interactable = true;
        }
    }

    private _onSignUp() {
        director.loadScene('register');
    }
}
