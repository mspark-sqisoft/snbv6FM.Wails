<!-- 디자인 고르기 단추(머리줄·로그인 화면). 지금 디자인에 체크가 붙는다. -->
<script lang="ts">
  import Palette from '@lucide/svelte/icons/palette'
  import Check from '@lucide/svelte/icons/check'
  import { ContextMenu, IconButton, type MenuItem } from '../lib/ui'
  import { t } from '../lib/i18n'
  import { design, setDesign } from '../lib/stores/ui'
  import { designs } from '../designs/registry'

  let open = $state(false)
  let x = $state(0)
  let y = $state(0)

  const items = $derived<MenuItem[]>(
    designs.map((d) => ({
      label: t(`design.${d.id}.name`),
      icon: d.id === $design ? Check : undefined,
      onSelect: () => setDesign(d.id),
    })),
  )

  function show(e: MouseEvent) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    x = r.right - 180
    y = r.bottom + 4
    open = true
  }
</script>

<IconButton icon={Palette} label="{t('design.menu')}: {t(`design.${$design}.name`)}" onclick={show} id="designMenuButton" />
<ContextMenu bind:open {x} {y} {items} label={t('design.menu')} />
