import { PlaceholderView } from '../../components/PlaceholderView';

@ComponentV2
export struct SearchPage {
  build() {
    PlaceholderView({ text: $r('app.string.page_pending_search') })
  }
}
