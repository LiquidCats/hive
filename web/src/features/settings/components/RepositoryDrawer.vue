<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { LoaderCircle } from '@lucide/vue'

import { fetchJSON } from '@/shared/composables/usePasskey'
import { Button } from '@/shared/ui/button'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '@/shared/ui/drawer'
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Switch } from '@/shared/ui/switch'
import { Textarea } from '@/shared/ui/textarea'

import { isSSH, type Repository } from './repository'

const props = defineProps<{ repo?: Repository }>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ saved: [] }>()

const url = ref('')
const name = ref('')
const username = ref('')
const password = ref('')
const sshKey = ref('')
const insecure = ref(false)
// Stored secret marked for removal; an empty input alone means "keep".
const clearPassword = ref(false)
const clearSshKey = ref(false)
const pending = ref(false)
const error = ref<string | null>(null)

const ssh = computed(() => isSSH(url.value))
const hasPassword = computed(() => !!props.repo?.hasPassword && !clearPassword.value)
const hasSshKey = computed(() => !!props.repo?.hasSshPrivateKey && !clearSshKey.value)

// Fresh state every time the drawer opens; the API never returns secrets, so nothing to prefill.
watch(open, (isOpen) => {
  if (!isOpen) return
  url.value = props.repo?.url ?? ''
  name.value = props.repo?.name ?? ''
  username.value = props.repo?.username ?? ''
  password.value = sshKey.value = ''
  insecure.value = props.repo?.insecure ?? false
  clearPassword.value = clearSshKey.value = false
  error.value = null
})

// API: omitted keeps the stored secret, "" clears it.
function secret(active: boolean, typed: string, cleared: boolean) {
  if (!active) return ''
  if (typed) return typed
  return cleared ? '' : undefined
}

async function submit() {
  pending.value = true
  error.value = null
  const body = {
    url: url.value.trim(),
    name: name.value.trim(),
    username: ssh.value ? '' : username.value.trim(),
    insecure: insecure.value,
    password: secret(!ssh.value, password.value, clearPassword.value),
    sshPrivateKey: secret(ssh.value, sshKey.value, clearSshKey.value),
  }
  try {
    await (props.repo
      ? fetchJSON(`/repositories/${props.repo.id}`, { method: 'PUT', body })
      : fetchJSON('/repositories', { method: 'POST', body }))
    open.value = false
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Request failed'
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <Drawer v-model:open="open">
    <DrawerContent class="max-w-xl mx-auto">
      <div
        class="mx-auto flex max-h-[85vh] w-full max-w-md flex-col gap-6 overflow-y-auto p-6 pb-10"
      >
        <DrawerHeader class="p-0">
          <DrawerTitle>{{ repo ? 'Edit repository' : 'Add repository' }}</DrawerTitle>
          <DrawerDescription>
            Hive checks the connection before saving, which can take a few seconds.
          </DrawerDescription>
        </DrawerHeader>

        <form @submit.prevent="submit">
          <FieldGroup>
            <Field>
              <FieldLabel for="repo-url">URL</FieldLabel>
              <Input
                id="repo-url"
                v-model="url"
                required
                placeholder="https://github.com/org/repo.git"
              />
              <FieldDescription v-if="repo">
                Changing the URL requires entering the credentials again.
              </FieldDescription>
            </Field>
            <Field>
              <FieldLabel for="repo-name">Name</FieldLabel>
              <Input id="repo-name" v-model="name" placeholder="Optional" />
            </Field>

            <template v-if="!ssh">
              <Field>
                <FieldLabel for="repo-username">Username</FieldLabel>
                <Input id="repo-username" v-model="username" autocomplete="off" />
              </Field>
              <Field>
                <FieldLabel for="repo-password">Password / token</FieldLabel>
                <Input
                  id="repo-password"
                  v-model="password"
                  type="password"
                  autocomplete="new-password"
                  :placeholder="hasPassword ? 'Stored — leave blank to keep' : ''"
                />
                <Button
                  v-if="hasPassword"
                  type="button"
                  variant="outline"
                  size="sm"
                  class="self-start"
                  @click="clearPassword = true"
                >
                  Remove
                </Button>
                <FieldDescription v-else-if="repo?.hasPassword">
                  The stored password will be removed on save.
                </FieldDescription>
              </Field>
            </template>
            <Field v-else>
              <FieldLabel for="repo-key">SSH private key</FieldLabel>
              <Textarea
                id="repo-key"
                v-model="sshKey"
                class="font-mono text-xs"
                rows="6"
                :placeholder="
                  hasSshKey ? 'Stored — leave blank to keep' : '-----BEGIN OPENSSH PRIVATE KEY-----'
                "
              />
              <Button
                v-if="hasSshKey"
                type="button"
                variant="outline"
                size="sm"
                class="self-start"
                @click="clearSshKey = true"
              >
                Remove
              </Button>
              <FieldDescription v-else-if="repo?.hasSshPrivateKey">
                The stored key will be removed on save.
              </FieldDescription>
            </Field>

            <Field orientation="horizontal">
              <Switch id="repo-insecure" v-model="insecure" />
              <FieldLabel for="repo-insecure">Skip TLS / host key verification</FieldLabel>
            </Field>

            <Field>
              <Button type="submit" size="lg" :disabled="pending">
                <LoaderCircle v-if="pending" class="animate-spin" />
                {{ pending ? 'Checking connection…' : repo ? 'Save changes' : 'Add repository' }}
              </Button>
              <FieldError :errors="[error ?? undefined]" role="alert" />
            </Field>
          </FieldGroup>
        </form>
      </div>
    </DrawerContent>
  </Drawer>
</template>
