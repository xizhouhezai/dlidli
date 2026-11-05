/// 应用外壳：底部导航（首页 / 我的）。
///
/// 为什么用 IndexedStack 而不是每次重建：切 Tab 不能丢首页的滚动位置与已加载分页，
/// 否则用户"看了 3 页视频、点开我的、再回来"会回到顶部，体验很差。
library;

import 'package:flutter/material.dart';

import '../features/feed/feed_page.dart';
import '../features/profile/profile_page.dart';

class RootShell extends StatefulWidget {
  const RootShell({super.key});

  @override
  State<RootShell> createState() => _RootShellState();
}

class _RootShellState extends State<RootShell> {
  int _index = 0;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: IndexedStack(
        index: _index,
        children: const [
          FeedPage(),
          ProfilePage(),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (i) => setState(() => _index = i),
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.home_outlined),
            selectedIcon: Icon(Icons.home),
            label: '首页',
          ),
          NavigationDestination(
            icon: Icon(Icons.person_outline),
            selectedIcon: Icon(Icons.person),
            label: '我的',
          ),
        ],
      ),
    );
  }
}
