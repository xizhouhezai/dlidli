import { pasteboard } from '@kit.BasicServicesKit';
import { ApiError } from '../model/Api';
import { CommentItem } from '../model/Interaction';
import { InteractionApi } from '../service/InteractionApi';
import { TokenStore } from '../store/TokenStore';
import { session } from '../store/Session';
import { formatPubdate } from '../common/utils/Format';
import { resolveMediaUrl } from '../common/utils/MediaUrl';
import { AppRouter, RouteName } from '../common/utils/AppRouter';
import { DliFont, DliRadius, DliSpace } from '../common/constants/Theme';
import { showToast } from '../common/utils/Toast';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'CommentSection';
const PAGE_SIZE: number = 20;
/** 楼中楼每次展开的条数（对齐 Web 端 expandReplies 的 10） */
const REPLY_PAGE_SIZE: number = 10;
/** 楼中楼默认预览条数（服务端一级评论附前 2 条） */
const REPLY_PREVIEW: number = 2;
/** 评论内容长度上限（服务端 binding 为 max=1000） */
const COMMENT_MAX_LEN: number = 1000;

/**
 * 评论区（M4-HMY-08 / CMT-01~04）：一级 + 二级评论、热度/最新排序、分页、楼中楼展开、
 * 发布/回复、点赞、删除（作者/UP 主）。举报走系统弹层，本期不做（归后台 ADT-10 链路）。
 *
 * 与 Web 端 CommentSection 同交互结构，差异在两点：
 * ① 点赞集合用 @Trace Set 做会话级乐观状态（Web 端是 ref(Set)）；
 * ② 楼中楼展开的加载更多沿用服务端 size 分页，页码按已加载条数推算。
 */
@ComponentV2
export struct CommentSection {
  @Require @Param bvid: string;

  @Local sort: 'hot' | 'new' = 'hot';
  @Local items: CommentItem[] = [];
  @Local total: number = 0;
  @Local loading: boolean = false;
  @Local posting: boolean = false;
  @Local input: string = '';
  /** 会话级已点赞评论 id（乐观态，页面切换不持久化） */
  @Local likedSet: Set<string> = new Set<string>();
  /** 回复态：正在回复的一级评论 id 与目标（一级或楼中楼） */
  @Local replyRootId: string = '';
  @Local replyParentId: string = '';
  @Local replyNickname: string = '';
  @Local replyInput: string = '';

  private page: number = 1;

  aboutToAppear(): void {
    this.load(true).catch((err: Object) => {
      Logger.warn(TAG, `评论加载失败：${JSON.stringify(err)}`);
    });
  }

  private loggedIn(): boolean {
    return TokenStore.isLoggedIn();
  }

  private requireLogin(): boolean {
    if (!this.loggedIn()) {
      showToast(this.getUIContext(), $r('app.string.cmt_need_login'));
      AppRouter.push(RouteName.LOGIN);
      return false;
    }
    return true;
  }

  private async load(reset: boolean): Promise<void> {
    if (this.loading) {
      return;
    }
    this.loading = true;
    try {
      if (reset) {
        this.page = 1;
      }
      const res = await InteractionApi.listCommentsPaged(this.bvid, this.sort, this.page, PAGE_SIZE);
      this.items = reset ? res.list : [...this.items, ...res.list];
      this.total = res.total;
    } catch (err) {
      showToast(this.getUIContext(), (err as ApiError).message);
    } finally {
      this.loading = false;
    }
  }

  private switchSort(s: 'hot' | 'new'): void {
    if (this.sort === s) {
      return;
    }
    this.sort = s;
    this.load(true).catch(() => {});
  }

  private loadMore(): void {
    this.page++;
    this.load(false).catch(() => {});
  }

  // ---- 发布 / 回复（CMT-01） ----

  private async postRoot(): Promise<void> {
    const content: string = this.input.trim();
    if (content.length === 0) {
      showToast(this.getUIContext(), $r('app.string.cmt_need_content'));
      return;
    }
    if (!this.requireLogin()) {
      return;
    }
    this.posting = true;
    try {
      const item = await InteractionApi.addComment(this.bvid, { content: content });
      this.items = [item, ...this.items];
      this.total++;
      this.input = '';
      showToast(this.getUIContext(), $r('app.string.cmt_sent'));
    } catch (err) {
      showToast(this.getUIContext(), (err as ApiError).message);
    } finally {
      this.posting = false;
    }
  }

  private openReply(root: CommentItem, target?: CommentItem): void {
    if (!this.requireLogin()) {
      return;
    }
    this.replyRootId = root.id;
    this.replyParentId = target?.id ?? root.id;
    this.replyNickname = (target ?? root).user.nickname;
    this.replyInput = '';
  }

  private closeReply(): void {
    this.replyRootId = '';
    this.replyParentId = '';
    this.replyInput = '';
  }

  private async postReply(root: CommentItem): Promise<void> {
    const content: string = this.replyInput.trim();
    if (content.length === 0) {
      showToast(this.getUIContext(), $r('app.string.cmt_need_content'));
      return;
    }
    if (!this.requireLogin()) {
      return;
    }
    this.posting = true;
    try {
      const item = await InteractionApi.addComment(this.bvid, {
        content: content,
        root_id: this.replyRootId,
        parent_id: this.replyParentId
      });
      // 本地挂载到楼中楼预览（Web 端同款；服务端已落库，重进时按真实分页补齐）
      const idx: number = this.items.findIndex((c: CommentItem) => c.id === root.id);
      if (idx >= 0) {
        const updated: CommentItem = this.clone(this.items[idx]);
        updated.replies = [...(updated.replies ?? []), item];
        updated.reply_cnt++;
        this.replaceRoot(idx, updated);
      }
      this.closeReply();
      showToast(this.getUIContext(), $r('app.string.cmt_sent'));
    } catch (err) {
      showToast(this.getUIContext(), (err as ApiError).message);
    } finally {
      this.posting = false;
    }
  }

  /** 在指定条目上做局部字段替换，返回一个新 CommentItem（ArkTS 不允许对 interface 用对象 spread） */
  private clone(c: CommentItem): CommentItem {
    return {
      id: c.id,
      content: c.content,
      user: c.user,
      like_cnt: c.like_cnt,
      reply_cnt: c.reply_cnt,
      is_top: c.is_top,
      is_self: c.is_self,
      created_at: c.created_at,
      replies: c.replies
    };
  }

  /** 替换一级列表里 idx 处的条目（触发 UI 刷新） */
  private replaceRoot(idx: number, updated: CommentItem): void {
    this.items = [...this.items.slice(0, idx), updated, ...this.items.slice(idx + 1)];
  }

  /** 替换一级评论 idx 处、其楼中楼 pIdx 处的条目 */
  private replaceReply(rootIdx: number, replyIdx: number, updated: CommentItem): void {
    const replies: CommentItem[] = this.items[rootIdx].replies ?? [];
    const newReplies: CommentItem[] = [
      ...replies.slice(0, replyIdx),
      updated,
      ...replies.slice(replyIdx + 1)
    ];
    const root: CommentItem = this.clone(this.items[rootIdx]);
    root.replies = newReplies;
    this.replaceRoot(rootIdx, root);
  }

  // ---- 点赞（ITR-02 / CMT-02） ----

  private async toggleLike(c: CommentItem, root?: CommentItem): Promise<void> {
    if (!this.requireLogin()) {
      return;
    }
    const before: boolean = this.likedSet.has(c.id);
    const beforeCnt: number = c.like_cnt;
    // 乐观更新
    const next: Set<string> = new Set<string>(this.likedSet);
    if (before) {
      next.delete(c.id);
    } else {
      next.add(c.id);
    }
    this.likedSet = next;
    this.setLikeCnt(c, root, beforeCnt + (before ? -1 : 1));
    try {
      const res = await InteractionApi.toggleCommentLike(c.id);
      // 按服务端回值对齐
      const aligned: Set<string> = new Set<string>(this.likedSet);
      if (res.liked) {
        aligned.add(c.id);
      } else {
        aligned.delete(c.id);
      }
      this.likedSet = aligned;
      this.setLikeCnt(c, root, beforeCnt + (res.liked === before ? 0 : res.liked ? 1 : -1));
    } catch (err) {
      // 失败回滚
      const rollback: Set<string> = new Set<string>(this.likedSet);
      if (before) {
        rollback.add(c.id);
      } else {
        rollback.delete(c.id);
      }
      this.likedSet = rollback;
      this.setLikeCnt(c, root, beforeCnt);
      showToast(this.getUIContext(), (err as ApiError).message);
    }
  }

  /** 写点赞计数到对应条目（一级或楼中楼），用 clone+replace 触发刷新（interface 不可用 spread） */
  private setLikeCnt(c: CommentItem, root: CommentItem | undefined, cnt: number): void {
    const value: number = Math.max(0, cnt);
    if (root === undefined) {
      const idx: number = this.items.findIndex((x: CommentItem) => x.id === c.id);
      if (idx >= 0) {
        const updated: CommentItem = this.clone(this.items[idx]);
        updated.like_cnt = value;
        this.replaceRoot(idx, updated);
      }
      return;
    }
    const rIdx: number = this.items.findIndex((x: CommentItem) => x.id === root.id);
    if (rIdx < 0) {
      return;
    }
    const replies: CommentItem[] = this.items[rIdx].replies ?? [];
    const pIdx: number = replies.findIndex((x: CommentItem) => x.id === c.id);
    if (pIdx < 0) {
      return;
    }
    const updated: CommentItem = this.clone(replies[pIdx]);
    updated.like_cnt = value;
    this.replaceReply(rIdx, pIdx, updated);
  }

  // ---- 删除（CMT-04） ----

  private async remove(c: CommentItem, root?: CommentItem): Promise<void> {
    try {
      await this.getUIContext().showAlertDialog({
        title: $r('app.string.cmt_delete_title'),
        message: $r('app.string.cmt_delete_msg'),
        autoCancel: true,
        alignment: DialogAlignment.Center,
        primaryButton: { value: $r('app.string.action_cancel'), action: () => {} },
        secondaryButton: { value: $r('app.string.action_delete'), action: () => {
          this.doRemove(c, root).catch((err: Object) => {
            Logger.warn(TAG, `删除评论失败：${JSON.stringify(err)}`);
          });
        } }
      });
    } catch (err) {
      Logger.warn(TAG, `删除确认弹层失败：${JSON.stringify(err)}`);
    }
  }

  private async doRemove(c: CommentItem, root?: CommentItem): Promise<void> {
    try {
      await InteractionApi.deleteComment(c.id);
      if (root === undefined) {
        this.items = this.items.filter((x: CommentItem) => x.id !== c.id);
        this.total = Math.max(0, this.total - 1);
      } else {
        const rIdx: number = this.items.findIndex((x: CommentItem) => x.id === root.id);
        if (rIdx >= 0) {
          const replies: CommentItem[] = (this.items[rIdx].replies ?? []).filter((x: CommentItem) => x.id !== c.id);
          const updated: CommentItem = this.clone(this.items[rIdx]);
          updated.replies = replies;
          updated.reply_cnt = Math.max(0, updated.reply_cnt - 1);
          this.replaceRoot(rIdx, updated);
        }
      }
      showToast(this.getUIContext(), $r('app.string.cmt_deleted'));
    } catch (err) {
      showToast(this.getUIContext(), (err as ApiError).message);
    }
  }

  // ---- 楼中楼展开（CMT-03） ----

  private async expandReplies(root: CommentItem): Promise<void> {
    const loaded: number = root.replies?.length ?? 0;
    try {
      const res = await InteractionApi.listReplies(root.id, Math.floor(loaded / REPLY_PAGE_SIZE) + 1, REPLY_PAGE_SIZE);
      const exist: Set<string> = new Set<string>((root.replies ?? []).map((r: CommentItem) => r.id));
      const fresh: CommentItem[] = res.list.filter((r: CommentItem) => !exist.has(r.id));
      const rIdx: number = this.items.findIndex((x: CommentItem) => x.id === root.id);
      if (rIdx >= 0) {
        const updated: CommentItem = this.clone(this.items[rIdx]);
        updated.replies = [...(updated.replies ?? []), ...fresh];
        this.replaceRoot(rIdx, updated);
      }
    } catch (err) {
      showToast(this.getUIContext(), (err as ApiError).message);
    }
  }

  private copyComment(c: CommentItem): void {
    const data: pasteboard.PasteData = pasteboard.createData(pasteboard.MIMETYPE_TEXT_PLAIN, c.content);
    pasteboard.getSystemPasteboard().setData(data).then(() => {
      showToast(this.getUIContext(), $r('app.string.dm_copied'));
    }).catch((err: Object) => {
      Logger.warn(TAG, `复制评论失败：${JSON.stringify(err)}`);
    });
  }

  // ---- UI ----

  @Builder
  header() {
    Row({ space: DliSpace.MD }) {
      Text($r('app.string.cmt_count', this.total))
        .fontSize(DliFont.BODY)
        .fontWeight(FontWeight.Medium)
        .fontColor($r('app.color.text_primary'))
      Text($r('app.string.cmt_sort_hot'))
        .fontSize(DliFont.CAPTION)
        .fontWeight(this.sort === 'hot' ? FontWeight.Medium : FontWeight.Normal)
        .fontColor(this.sort === 'hot' ? $r('app.color.brand_primary') : $r('app.color.text_tertiary'))
        .onClick(() => {
          this.switchSort('hot');
        })
      Text('|')
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.border_default'))
      Text($r('app.string.cmt_sort_new'))
        .fontSize(DliFont.CAPTION)
        .fontWeight(this.sort === 'new' ? FontWeight.Medium : FontWeight.Normal)
        .fontColor(this.sort === 'new' ? $r('app.color.brand_primary') : $r('app.color.text_tertiary'))
        .onClick(() => {
          this.switchSort('new');
        })
    }
    .width('100%')
    .alignItems(VerticalAlign.Center)
  }

  @Builder
  postBox() {
    Row({ space: DliSpace.SM }) {
      TextInput({ text: $$this.input, placeholder: $r('app.string.cmt_input_hint') })
        .maxLength(COMMENT_MAX_LEN)
        .fontSize(DliFont.BODY_SM)
        .layoutWeight(1)
      Button($r('app.string.cmt_send'))
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.text_on_brand'))
        .backgroundColor($r('app.color.brand_primary'))
        .borderRadius(DliRadius.FULL)
        .enabled(!this.posting)
        .onClick(() => {
          this.postRoot();
        })
    }
    .width('100%')
    .alignItems(VerticalAlign.Center)
  }

  @Builder
  replyBox(root: CommentItem) {
    Row({ space: DliSpace.SM }) {
      TextInput({ text: $$this.replyInput, placeholder: $r('app.string.cmt_reply_hint', this.replyNickname) })
        .maxLength(COMMENT_MAX_LEN)
        .fontSize(DliFont.BODY_SM)
        .layoutWeight(1)
      Button($r('app.string.cmt_send'))
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_on_brand'))
        .backgroundColor($r('app.color.brand_primary'))
        .borderRadius(DliRadius.FULL)
        .enabled(!this.posting)
        .onClick(() => {
          this.postReply(root);
        })
      Button($r('app.string.action_cancel'))
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_secondary'))
        .backgroundColor($r('app.color.surface_secondary'))
        .borderRadius(DliRadius.FULL)
        .onClick(() => {
          this.closeReply();
        })
    }
    .width('100%')
    .alignItems(VerticalAlign.Center)
  }

  @Builder
  commentRow(c: CommentItem, root?: CommentItem) {
    Row({ space: DliSpace.SM }) {
      if (c.user.avatar.length > 0) {
        Image(resolveMediaUrl(c.user.avatar))
          .width(32)
          .height(32)
          .borderRadius(DliRadius.FULL)
          .objectFit(ImageFit.Cover)
      } else {
        Text(c.user.nickname.length > 0 ? c.user.nickname.substring(0, 1) : 'U')
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_on_brand'))
          .textAlign(TextAlign.Center)
          .width(32)
          .height(32)
          .borderRadius(DliRadius.FULL)
          .backgroundColor($r('app.color.brand_primary'))
      }
      Column({ space: 2 }) {
        Row({ space: DliSpace.XS }) {
          Text(c.user.nickname)
            .fontSize(DliFont.CAPTION)
            .fontWeight(FontWeight.Medium)
            .fontColor($r('app.color.text_secondary'))
          if (c.is_top) {
            Text($r('app.string.cmt_top'))
              .fontSize(10)
              .fontColor($r('app.color.brand_primary'))
              .padding({ left: 4, right: 4, top: 1, bottom: 1 })
              .borderRadius(DliRadius.SM)
              .border({ width: 1, color: $r('app.color.brand_primary') })
          }
        }
        Text(c.content)
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_primary'))
          .width('100%')
        Row({ space: DliSpace.LG }) {
          Text(formatPubdate(c.created_at))
            .fontSize(DliFont.CAPTION)
            .fontColor($r('app.color.text_tertiary'))
          Row({ space: 2 }) {
            SymbolGlyph(this.likedSet.has(c.id) ? $r('sys.symbol.heart_fill') : $r('sys.symbol.heart'))
              .fontSize(13)
              .fontColor([this.likedSet.has(c.id) ? $r('app.color.brand_primary') : $r('app.color.text_tertiary')])
            if (c.like_cnt > 0) {
              Text(`${c.like_cnt}`)
                .fontSize(DliFont.CAPTION)
                .fontColor(this.likedSet.has(c.id) ? $r('app.color.brand_primary') : $r('app.color.text_tertiary'))
            }
          }
          .onClick(() => {
            this.toggleLike(c, root);
          })
          Text($r('app.string.cmt_reply'))
            .fontSize(DliFont.CAPTION)
            .fontColor($r('app.color.text_tertiary'))
            .onClick(() => {
              this.openReply(root ?? c, root === undefined ? undefined : c);
            })
          if (c.is_self) {
            Text($r('app.string.action_delete'))
              .fontSize(DliFont.CAPTION)
              .fontColor($r('app.color.text_tertiary'))
              .onClick(() => {
                this.remove(c, root);
              })
          }
          Text($r('app.string.dm_action_copy'))
            .fontSize(DliFont.CAPTION)
            .fontColor($r('app.color.text_tertiary'))
            .onClick(() => {
              this.copyComment(c);
            })
        }
        .alignItems(VerticalAlign.Center)
      }
      .layoutWeight(1)
      .alignItems(HorizontalAlign.Start)
    }
    .width('100%')
    .alignItems(VerticalAlign.Top)
    .padding({ top: DliSpace.SM, bottom: DliSpace.SM })
  }

  @Builder
  repliesBlock(root: CommentItem) {
    Column({ space: DliSpace.XS }) {
      ForEach(root.replies ?? [], (r: CommentItem) => {
        Row({ space: DliSpace.SM }) {
          Column({ space: 2 }) {
            Row({ space: DliSpace.XS }) {
              Text(r.user.nickname)
                .fontSize(DliFont.CAPTION)
                .fontWeight(FontWeight.Medium)
                .fontColor($r('app.color.text_secondary'))
            }
            Text(r.content)
              .fontSize(DliFont.BODY_SM)
              .fontColor($r('app.color.text_primary'))
              .width('100%')
            Row({ space: DliSpace.LG }) {
              Text(formatPubdate(r.created_at))
                .fontSize(DliFont.CAPTION)
                .fontColor($r('app.color.text_tertiary'))
              Row({ space: 2 }) {
                SymbolGlyph(this.likedSet.has(r.id) ? $r('sys.symbol.heart_fill') : $r('sys.symbol.heart'))
                  .fontSize(12)
                  .fontColor([this.likedSet.has(r.id) ? $r('app.color.brand_primary') : $r('app.color.text_tertiary')])
                if (r.like_cnt > 0) {
                  Text(`${r.like_cnt}`)
                    .fontSize(DliFont.CAPTION)
                    .fontColor(this.likedSet.has(r.id) ? $r('app.color.brand_primary') : $r('app.color.text_tertiary'))
                }
              }
              .onClick(() => {
                this.toggleLike(r, root);
              })
              Text($r('app.string.cmt_reply'))
                .fontSize(DliFont.CAPTION)
                .fontColor($r('app.color.text_tertiary'))
                .onClick(() => {
                  this.openReply(root, r);
                })
              if (r.is_self) {
                Text($r('app.string.action_delete'))
                  .fontSize(DliFont.CAPTION)
                  .fontColor($r('app.color.text_tertiary'))
                  .onClick(() => {
                    this.remove(r, root);
                  })
              }
            }
            .alignItems(VerticalAlign.Center)
          }
          .layoutWeight(1)
          .alignItems(HorizontalAlign.Start)
        }
        .width('100%')
        .alignItems(VerticalAlign.Top)
      }, (r: CommentItem) => r.id)

      if ((root.replies?.length ?? 0) < root.reply_cnt) {
        Text($r('app.string.cmt_expand_more', root.reply_cnt))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.brand_primary'))
          .onClick(() => {
            this.expandReplies(root);
          })
      }
    }
    .width('100%')
    .alignItems(HorizontalAlign.Start)
    .padding(DliSpace.SM)
    .borderRadius(DliRadius.SM)
    .backgroundColor($r('app.color.surface_secondary'))
  }

  build() {
    Column({ space: DliSpace.MD }) {
      this.header()
      this.postBox()

      if (this.items.length === 0 && !this.loading) {
        Text($r('app.string.cmt_empty'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_tertiary'))
          .width('100%')
          .textAlign(TextAlign.Center)
          .padding({ top: DliSpace.LG, bottom: DliSpace.LG })
      }

      ForEach(this.items, (c: CommentItem) => {
        Column({ space: DliSpace.SM }) {
          this.commentRow(c)
          if (this.replyRootId === c.id) {
            this.replyBox(c)
          }
          if ((c.replies?.length ?? 0) > 0) {
            this.repliesBlock(c)
          }
        }
        .width('100%')
        .alignItems(HorizontalAlign.Start)
      }, (c: CommentItem) => c.id)

      if (this.items.length < this.total) {
        Button(this.loading ? $r('app.string.feed_loading_more') : $r('app.string.cmt_load_more'))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_primary'))
          .backgroundColor($r('app.color.surface_secondary'))
          .borderRadius(DliRadius.FULL)
          .enabled(!this.loading)
          .onClick(() => {
            this.loadMore();
          })
      }
    }
    .width('100%')
    .alignItems(HorizontalAlign.Start)
  }
}
