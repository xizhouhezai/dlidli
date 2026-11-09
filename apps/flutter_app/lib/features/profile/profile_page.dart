/// 个人中心（T-03 的最小载体：展示登录态 + 退出登录）。
///
/// 「我的投稿 / 观看历史 / 收藏」三档分页属于 T-42，此处**不假装完成**，
/// 只把 T-03 的验收链路（登录态可视化 + 退出清理）做通。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/auth.dart';
import '../../data/models/account.dart';
import '../auth/login_page.dart';

class ProfilePage extends ConsumerWidget {
  const ProfilePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('我的')),
      body: switch (auth) {
        AuthUnknown() => const Center(child: CircularProgressIndicator()),
        AuthLoggedOut(:final reason) => _GuestView(reason: reason),
        AuthLoggedIn(:final profile) => _LoggedInView(profile: profile),
      },
    );
  }
}

class _GuestView extends StatelessWidget {
  const _GuestView({this.reason});

  final String? reason;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const CircleAvatar(radius: 36, child: Icon(Icons.person, size: 36)),
          const SizedBox(height: 16),
          const Text('登录后查看更多内容'),
          if (reason != null) ...[
            const SizedBox(height: 6),
            Text(
              reason!,
              style: const TextStyle(fontSize: 12, color: Colors.grey),
            ),
          ],
          const SizedBox(height: 20),
          FilledButton(
            onPressed:
                () => Navigator.of(context).push(
                  MaterialPageRoute<bool>(builder: (_) => const LoginPage()),
                ),
            child: const Text('登录 / 注册'),
          ),
        ],
      ),
    );
  }
}

class _LoggedInView extends ConsumerWidget {
  const _LoggedInView({required this.profile});

  final Profile profile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final subtitle = <String>[
      'Lv${profile.level}',
      if (profile.isVip) '大会员',
      if (profile.signature.isNotEmpty) profile.signature,
    ].join(' · ');

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Card(
          child: ListTile(
            leading: CircleAvatar(
              radius: 28,
              backgroundImage:
                  profile.avatar.isEmpty ? null : NetworkImage(profile.avatar),
              child: profile.avatar.isEmpty ? const Icon(Icons.person) : null,
            ),
            title: Text(profile.nickname),
            subtitle: Text(subtitle),
          ),
        ),
        const SizedBox(height: 12),
        const Card(
          child: ListTile(
            leading: Icon(Icons.video_library_outlined),
            title: Text('我的投稿'),
            subtitle: Text('由 T-42 提供'),
          ),
        ),
        const Card(
          child: ListTile(
            leading: Icon(Icons.history),
            title: Text('观看历史'),
            subtitle: Text('由 T-42 提供'),
          ),
        ),
        const Card(
          child: ListTile(
            leading: Icon(Icons.star_border),
            title: Text('我的收藏'),
            subtitle: Text('由 T-42 提供'),
          ),
        ),
        const SizedBox(height: 24),
        OutlinedButton.icon(
          onPressed: () => _confirmLogout(context, ref),
          icon: const Icon(Icons.logout),
          label: const Text('退出登录'),
        ),
      ],
    );
  }

  Future<void> _confirmLogout(BuildContext context, WidgetRef ref) async {
    final ok = await showDialog<bool>(
      context: context,
      builder:
          (ctx) => AlertDialog(
            title: const Text('退出登录'),
            content: const Text('确定要退出当前账号吗？'),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(ctx, false),
                child: const Text('取消'),
              ),
              FilledButton(
                onPressed: () => Navigator.pop(ctx, true),
                child: const Text('退出'),
              ),
            ],
          ),
    );
    if (ok == true) {
      await ref.read(authProvider.notifier).logout();
    }
  }
}
