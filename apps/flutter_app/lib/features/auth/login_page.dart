/// 登录页（T-03 / FLT-01）。
///
/// 本期做**验证码登录**（主路径）：手机号 + 6 位验证码。
/// 密码登录需要图形验证码（后端 `captcha_id`+`captcha_code` 必填，SVG 由端侧渲染），
/// 属于 T-03 的次要路径，放在二期补（已在 tasks 里登记为未覆盖，不假装做完）。
///
/// **dev 便利**：后端在 dev 环境把验证码放在 `debug_code` 里返回，
/// 本页在「拿到 debug_code」时自动填入并提示 —— 生产环境该字段不存在，逻辑自然失效。
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/auth.dart';
import '../../core/api_error.dart';

class LoginPage extends ConsumerStatefulWidget {
  const LoginPage({super.key});

  @override
  ConsumerState<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends ConsumerState<LoginPage> {
  final _phone = TextEditingController();
  final _code = TextEditingController();
  final _formKey = GlobalKey<FormState>();

  bool _submitting = false;
  bool _sending = false;
  int _countdown = 0;
  String? _hint;
  String? _error;

  @override
  void dispose() {
    _phone.dispose();
    _code.dispose();
    super.dispose();
  }

  Future<void> _sendCode() async {
    final phone = _phone.text.trim();
    if (!RegExp(r'^1[3-9]\d{9}$').hasMatch(phone)) {
      setState(() => _error = '请输入正确的手机号');
      return;
    }
    setState(() {
      _sending = true;
      _error = null;
      _hint = null;
    });
    try {
      final api = ref.read(authApiProvider);
      final debugCode = await api.sendSmsCode(phone);
      if (!mounted) return;
      setState(() {
        _sending = false;
        _countdown = 60;
        if (debugCode != null && debugCode.isNotEmpty) {
          // dev 环境：后端把验证码返回给我们，直接填好并提示
          _code.text = debugCode;
          _hint = '开发环境：验证码已自动填入（$debugCode）';
        } else {
          _hint = '验证码已发送，请查收短信';
        }
      });
      _startCountdown();
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _sending = false;
        _error = e.message;
      });
    }
  }

  void _startCountdown() {
    Future.doWhile(() async {
      await Future<void>.delayed(const Duration(seconds: 1));
      if (!mounted) return false;
      if (_countdown <= 1) {
        setState(() => _countdown = 0);
        return false;
      }
      setState(() => _countdown--);
      return true;
    });
  }

  Future<void> _submit() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await ref.read(authProvider.notifier).loginBySms(
            phone: _phone.text.trim(),
            code: _code.text.trim(),
          );
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _submitting = false;
        _error = e.message;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _submitting = false;
        _error = '登录失败，请稍后重试';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('登录')),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const SizedBox(height: 24),
                Text(
                  '欢迎回到 DliDli',
                  style: Theme.of(context).textTheme.headlineSmall,
                ),
                const SizedBox(height: 8),
                const Text('未注册的手机号将自动创建账号'),
                const SizedBox(height: 32),
                TextFormField(
                  controller: _phone,
                  keyboardType: TextInputType.phone,
                  inputFormatters: [
                    FilteringTextInputFormatter.digitsOnly,
                    LengthLimitingTextInputFormatter(11),
                  ],
                  decoration: const InputDecoration(
                    labelText: '手机号',
                    prefixIcon: Icon(Icons.phone_iphone),
                    border: OutlineInputBorder(),
                  ),
                  validator: (v) {
                    final s = (v ?? '').trim();
                    if (s.isEmpty) return '请输入手机号';
                    if (!RegExp(r'^1[3-9]\d{9}$').hasMatch(s)) return '手机号格式不正确';
                    return null;
                  },
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    Expanded(
                      child: TextFormField(
                        controller: _code,
                        keyboardType: TextInputType.number,
                        inputFormatters: [
                          FilteringTextInputFormatter.digitsOnly,
                          LengthLimitingTextInputFormatter(6),
                        ],
                        decoration: const InputDecoration(
                          labelText: '验证码',
                          prefixIcon: Icon(Icons.password),
                          border: OutlineInputBorder(),
                        ),
                        validator: (v) {
                          final s = (v ?? '').trim();
                          if (s.length != 6) return '请输入 6 位验证码';
                          return null;
                        },
                      ),
                    ),
                    const SizedBox(width: 12),
                    SizedBox(
                      height: 56,
                      child: OutlinedButton(
                        onPressed: (_sending || _countdown > 0) ? null : _sendCode,
                        child: Text(
                          _countdown > 0 ? '${_countdown}s' : (_sending ? '发送中' : '获取验证码'),
                        ),
                      ),
                    ),
                  ],
                ),
                if (_hint != null) ...[
                  const SizedBox(height: 12),
                  Text(_hint!, style: const TextStyle(color: Colors.blue, fontSize: 12)),
                ],
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Text(_error!, style: const TextStyle(color: Colors.red, fontSize: 13)),
                ],
                const SizedBox(height: 24),
                SizedBox(
                  height: 48,
                  child: FilledButton(
                    onPressed: _submitting ? null : _submit,
                    child: _submitting
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('登录'),
                  ),
                ),
                const SizedBox(height: 16),
                TextButton(
                  onPressed: () => ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('密码登录需要图形验证码，将在后续版本提供')),
                  ),
                  child: const Text('使用密码登录'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
