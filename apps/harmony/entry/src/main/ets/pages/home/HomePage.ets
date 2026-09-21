import { PlaceholderView } from '../../components/PlaceholderView';

@ComponentV2
export struct HomePage {
  build() {
    PlaceholderView({ text: $r('app.string.page_pending_home') })
  }
}
