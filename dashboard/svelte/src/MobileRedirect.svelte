<script>
  import { mobileRedirectMessage } from './lib/layout.js';

  let { view } = $props();
</script>

<div class="mobile-redirect" data-testid="mobile-redirect">
  <p>{mobileRedirectMessage(view)}</p>
</div>

<style>
  .mobile-redirect {
    padding: 2rem 1rem;
    margin: 2rem 1rem;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    text-align: center;
    color: #e0e0e0;
  }
  .mobile-redirect p {
    margin: 0;
    font-size: 1rem;
    line-height: 1.5;
  }
</style>
