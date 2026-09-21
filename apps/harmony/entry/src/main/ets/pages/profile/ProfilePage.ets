import { PlaceholderView } from '../../components/PlaceholderView';

@ComponentV2
export struct ProfilePage {
  build() {
    PlaceholderView({ text: $r('app.string.page_pending_profile') })
  }
}
