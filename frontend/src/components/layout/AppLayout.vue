<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useContactsStore } from '@/stores/contacts'
import { usersService, chatbotService } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Separator } from '@/components/ui/separator'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from '@/components/ui/popover'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import {
  LayoutDashboard,
  MessageSquare,
  Bot,
  FileText,
  Megaphone,
  Settings,
  LogOut,
  ChevronLeft,
  ChevronRight,
  Users,
  Workflow,
  Sparkles,
  Key,
  User,
  UserX,
  MessageSquareText,
  Sun,
  Moon,
  Monitor,
  Webhook,
  BarChart3,
  ShieldCheck,
  Zap
} from 'lucide-vue-next'
import { useColorMode } from '@/composables/useColorMode'
import { toast } from 'vue-sonner'
import { getInitials } from '@/lib/utils'
import { wsService } from '@/services/websocket'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const contactsStore = useContactsStore()
const isCollapsed = ref(false)
const isUserMenuOpen = ref(false)
const isUpdatingAvailability = ref(false)
const isCheckingTransfers = ref(false)
const showAwayWarning = ref(false)
const awayWarningTransferCount = ref(0)
const { colorMode, isDark, setColorMode } = useColorMode()

const handleAvailabilityChange = async (checked: boolean) => {
  // If going away, check for assigned transfers first
  if (!checked) {
    isCheckingTransfers.value = true
    try {
      // Fetch current user's active transfers from API
      const response = await chatbotService.listTransfers({ status: 'active' })
      const data = response.data.data || response.data
      const transfers = data.transfers || []
      const userId = authStore.user?.id
      const myActiveTransfers = transfers.filter((t: any) => t.agent_id === userId)

      if (myActiveTransfers.length > 0) {
        awayWarningTransferCount.value = myActiveTransfers.length
        showAwayWarning.value = true
        return
      }
    } catch (error) {
      console.error('Failed to check transfers:', error)
      // Proceed anyway if check fails
    } finally {
      isCheckingTransfers.value = false
    }
  }

  await setAvailability(checked)
}

const confirmGoAway = async () => {
  showAwayWarning.value = false
  await setAvailability(false)
}

const setAvailability = async (checked: boolean) => {
  isUpdatingAvailability.value = true
  try {
    const response = await usersService.updateAvailability(checked)
    const data = response.data.data
    authStore.setAvailability(checked, data.break_started_at)

    if (checked) {
      toast.success('Available', {
        description: 'You are now available to receive transfers'
      })
    } else {
      const transfersReturned = data.transfers_to_queue || 0
      toast.success('Away', {
        description: transfersReturned > 0
          ? `${transfersReturned} transfer(s) returned to queue`
          : 'You will not receive new transfer assignments'
      })

      // Refresh contacts list if transfers were returned to queue
      if (transfersReturned > 0) {
        contactsStore.fetchContacts()
      }
    }
  } catch (error) {
    toast.error('Error', {
      description: 'Failed to update availability'
    })
  } finally {
    isUpdatingAvailability.value = false
  }
}

// Calculate break duration for display
const breakDuration = ref('')
let breakTimerInterval: ReturnType<typeof setInterval> | null = null

const updateBreakDuration = () => {
  if (!authStore.breakStartedAt) {
    breakDuration.value = ''
    return
  }
  const start = new Date(authStore.breakStartedAt)
  const now = new Date()
  const diffMs = now.getTime() - start.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const hours = Math.floor(diffMins / 60)
  const mins = diffMins % 60

  if (hours > 0) {
    breakDuration.value = `${hours}h ${mins}m`
  } else {
    breakDuration.value = `${mins}m`
  }
}

// Start/stop break timer based on availability
watch(() => authStore.isAvailable, (available) => {
  if (!available && authStore.breakStartedAt) {
    updateBreakDuration()
    breakTimerInterval = setInterval(updateBreakDuration, 60000) // Update every minute
  } else if (breakTimerInterval) {
    clearInterval(breakTimerInterval)
    breakTimerInterval = null
    breakDuration.value = ''
  }
}, { immediate: true })

// Restore break time on mount and connect WebSocket
onMounted(() => {
  authStore.restoreBreakTime()
  if (!authStore.isAvailable && authStore.breakStartedAt) {
    updateBreakDuration()
    breakTimerInterval = setInterval(updateBreakDuration, 60000)
  }

  // Connect WebSocket for real-time updates across all pages
  const token = localStorage.getItem('auth_token')
  if (token) {
    wsService.connect(token)
  }
})

onUnmounted(() => {
  if (breakTimerInterval) {
    clearInterval(breakTimerInterval)
  }
})

// Define all navigation items with role requirements
const allNavItems = [
  {
    name: 'Dashboard',
    path: '/',
    icon: LayoutDashboard,
    roles: ['admin', 'manager']
  },
  {
    name: 'Chat',
    path: '/chat',
    icon: MessageSquare,
    roles: ['admin', 'manager', 'agent']
  },
  {
    name: 'Chatbot',
    path: '/chatbot',
    icon: Bot,
    roles: ['admin', 'manager'],
    children: [
      { name: 'Overview', path: '/chatbot', icon: Bot },
      { name: 'Keywords', path: '/chatbot/keywords', icon: Key },
      { name: 'Flows', path: '/chatbot/flows', icon: Workflow },
      { name: 'AI Contexts', path: '/chatbot/ai', icon: Sparkles }
    ]
  },
  {
    name: 'Transfers',
    path: '/chatbot/transfers',
    icon: UserX,
    roles: ['admin', 'manager', 'agent']
  },
  {
    name: 'Agent Analytics',
    path: '/analytics/agents',
    icon: BarChart3,
    roles: ['admin', 'manager', 'agent']
  },
  {
    name: 'Templates',
    path: '/templates',
    icon: FileText,
    roles: ['admin', 'manager']
  },
  {
    name: 'Flows',
    path: '/flows',
    icon: Workflow,
    roles: ['admin', 'manager']
  },
  {
    name: 'Campaigns',
    path: '/campaigns',
    icon: Megaphone,
    roles: ['admin', 'manager']
  },
  {
    name: 'Settings',
    path: '/settings',
    icon: Settings,
    roles: ['admin', 'manager'],
    children: [
      { name: 'General', path: '/settings', icon: Settings },
      { name: 'Chatbot', path: '/settings/chatbot', icon: Bot },
      { name: 'Accounts', path: '/settings/accounts', icon: Users },
      { name: 'Canned Responses', path: '/settings/canned-responses', icon: MessageSquareText },
      { name: 'Teams', path: '/settings/teams', icon: Users },
      { name: 'Users', path: '/settings/users', icon: Users, roles: ['admin'] },
      { name: 'API Keys', path: '/settings/api-keys', icon: Key, roles: ['admin'] },
      { name: 'Webhooks', path: '/settings/webhooks', icon: Webhook, roles: ['admin'] },
      { name: 'Custom Actions', path: '/settings/custom-actions', icon: Zap, roles: ['admin'] },
      { name: 'SSO', path: '/settings/sso', icon: ShieldCheck, roles: ['admin'] }
    ]
  }
]

// Filter navigation based on user role
const navigation = computed(() => {
  const userRole = authStore.userRole || 'agent'

  return allNavItems
    .filter(item => item.roles.includes(userRole))
    .map(item => ({
      ...item,
      active: item.path === '/'
        ? route.name === 'dashboard'
        : item.path === '/chat'
          ? route.name === 'chat' || route.name === 'chat-conversation'
          : route.path.startsWith(item.path),
      children: item.children?.filter(
        child => !child.roles || child.roles.includes(userRole)
      )
    }))
})

const toggleSidebar = () => {
  isCollapsed.value = !isCollapsed.value
}

const handleLogout = async () => {
  await authStore.logout()
  router.push('/login')
}
</script>

<template>
  <div class="flex h-screen bg-background">
    <!-- Sidebar -->
    <aside
      :class="[
        'flex flex-col border-r bg-card transition-all duration-300',
        isCollapsed ? 'w-16' : 'w-64'
      ]"
    >
      <!-- Logo -->
      <div class="flex h-12 items-center justify-between px-3 border-b">
        <RouterLink to="/" class="flex items-center gap-2">
          <svg v-if="!isCollapsed" width="28" height="28" viewBox="0 0 512 512" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M502.724 255.061c0 45.086-10.958 86.102-32.562 122.421-21.604 36.632-51.035 65.437-88.294 86.728s-78.901 31.936-125.552 31.936-88.294-10.645-125.553-31.936-66.69-50.096-88.606-86.728C20.24 340.849 9.28 300.147 9.28 255.061s10.959-85.789 32.562-121.483C63.447 97.885 92.88 69.08 130.45 47.79q56.358-31.936 124.926-31.936c45.713 0 89.233 10.645 126.179 31.935 37.259 21.291 66.69 49.783 88.294 85.789s32.562 76.396 32.562 121.483z" fill="#47b772"/>
            <path d="M116.361 256.939c0-24.422 5.322-45.712 16.594-64.498 11.584-19.412 26.613-34.441 46.651-45.712 18.786-10.646 40.077-16.282 64.499-16.908h144.338l-47.278 35.067c8.767 8.141 16.281 17.534 22.856 28.179 11.272 18.786 16.594 39.45 16.594 63.872s-5.322 44.773-16.281 62.62c-11.584 18.786-26.926 33.814-47.277 45.399-20.039 11.585-42.582 17.22-68.569 17.22s-49.783-5.635-69.508-16.907c-20.038-11.584-35.693-26.3-47.278-45.712-10.645-17.847-15.968-38.198-15.968-62.62z" fill="#fff"/>
            <path d="M193.061 273.532c9.683 0 17.533-7.85 17.533-17.533s-7.85-17.534-17.533-17.534-17.534 7.85-17.534 17.534 7.85 17.533 17.534 17.533m57.605 0c9.684 0 17.534-7.85 17.534-17.533s-7.85-17.534-17.534-17.534-17.533 7.85-17.533 17.534 7.85 17.533 17.533 17.533m57.617 0c9.684 0 17.534-7.85 17.534-17.533s-7.85-17.534-17.534-17.534-17.533 7.85-17.533 17.534 7.85 17.533 17.533 17.533" fill="#47b772"/>
          </svg>
          <svg v-else width="28" height="28" viewBox="0 0 512 512" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M502.724 255.061c0 45.086-10.958 86.102-32.562 122.421-21.604 36.632-51.035 65.437-88.294 86.728s-78.901 31.936-125.552 31.936-88.294-10.645-125.553-31.936-66.69-50.096-88.606-86.728C20.24 340.849 9.28 300.147 9.28 255.061s10.959-85.789 32.562-121.483C63.447 97.885 92.88 69.08 130.45 47.79q56.358-31.936 124.926-31.936c45.713 0 89.233 10.645 126.179 31.935 37.259 21.291 66.69 49.783 88.294 85.789s32.562 76.396 32.562 121.483z" fill="#47b772"/>
            <path d="M116.361 256.939c0-24.422 5.322-45.712 16.594-64.498 11.584-19.412 26.613-34.441 46.651-45.712 18.786-10.646 40.077-16.282 64.499-16.908h144.338l-47.278 35.067c8.767 8.141 16.281 17.534 22.856 28.179 11.272 18.786 16.594 39.45 16.594 63.872s-5.322 44.773-16.281 62.62c-11.584 18.786-26.926 33.814-47.277 45.399-20.039 11.585-42.582 17.22-68.569 17.22s-49.783-5.635-69.508-16.907c-20.038-11.584-35.693-26.3-47.278-45.712-10.645-17.847-15.968-38.198-15.968-62.62z" fill="#fff"/>
            <path d="M193.061 273.532c9.683 0 17.533-7.85 17.533-17.533s-7.85-17.534-17.533-17.534-17.534 7.85-17.534 17.534 7.85 17.533 17.534 17.533m57.605 0c9.684 0 17.534-7.85 17.534-17.533s-7.85-17.534-17.534-17.534-17.533 7.85-17.533 17.534 7.85 17.533 17.533 17.533m57.617 0c9.684 0 17.534-7.85 17.534-17.533s-7.85-17.534-17.534-17.534-17.533 7.85-17.533 17.534 7.85 17.533 17.533 17.533" fill="#47b772"/>
          </svg>
          <span
            v-if="!isCollapsed"
            class="font-semibold text-sm text-foreground"
          >
            XOBITO
          </span>
        </RouterLink>
        <Button
          variant="ghost"
          size="icon"
          class="h-7 w-7"
          @click="toggleSidebar"
        >
          <ChevronLeft v-if="!isCollapsed" class="h-3.5 w-3.5" />
          <ChevronRight v-else class="h-3.5 w-3.5" />
        </Button>
      </div>

      <!-- Navigation -->
      <ScrollArea class="flex-1 py-2">
        <nav class="space-y-0.5 px-2">
          <template v-for="item in navigation" :key="item.path">
            <RouterLink
              :to="item.path"
              :class="[
                'flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13px] font-medium transition-colors',
                item.active
                  ? 'bg-primary/10 text-primary'
                  : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                isCollapsed && 'justify-center px-2'
              ]"
            >
              <component :is="item.icon" class="h-4 w-4 shrink-0" />
              <span v-if="!isCollapsed">{{ item.name }}</span>
            </RouterLink>

            <!-- Submenu items -->
            <template v-if="item.children && item.active && !isCollapsed">
              <RouterLink
                v-for="child in item.children"
                :key="child.path"
                :to="child.path"
                :class="[
                  'flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13px] font-medium transition-colors ml-4',
                  route.path === child.path
                    ? 'bg-primary/10 text-primary'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                ]"
              >
                <component :is="child.icon" class="h-3.5 w-3.5 shrink-0" />
                <span>{{ child.name }}</span>
              </RouterLink>
            </template>
          </template>
        </nav>
      </ScrollArea>

      <!-- User section -->
      <div class="border-t p-2">
        <Popover v-model:open="isUserMenuOpen">
          <PopoverTrigger as-child>
            <Button
              variant="ghost"
              :class="[
                'flex items-center w-full h-auto px-2 py-1.5 gap-2',
                isCollapsed && 'justify-center'
              ]"
            >
              <Avatar class="h-7 w-7">
                <AvatarImage :src="undefined" />
                <AvatarFallback class="text-xs">
                  {{ getInitials(authStore.user?.full_name || 'U') }}
                </AvatarFallback>
              </Avatar>
              <div v-if="!isCollapsed" class="flex flex-col items-start text-left">
                <span class="text-[13px] font-medium truncate max-w-[140px]">
                  {{ authStore.user?.full_name }}
                </span>
                <span class="text-[11px] text-muted-foreground truncate max-w-[140px]">
                  {{ authStore.user?.email }}
                </span>
              </div>
            </Button>
          </PopoverTrigger>
          <PopoverContent side="top" align="start" class="w-52 p-1.5">
            <div class="text-xs font-medium px-2 py-1 text-muted-foreground">My Account</div>
            <Separator class="my-1" />
            <!-- Availability Toggle -->
            <div class="flex items-center justify-between px-2 py-1.5">
              <div class="flex items-center gap-2">
                <span class="text-[13px]">Status</span>
                <Badge :variant="authStore.isAvailable ? 'default' : 'secondary'" class="text-[10px] px-1.5 py-0">
                  {{ authStore.isAvailable ? 'Available' : 'Away' }}
                </Badge>
                <span v-if="!authStore.isAvailable && breakDuration" class="text-[10px] text-muted-foreground">
                  {{ breakDuration }}
                </span>
              </div>
              <Switch
                :checked="authStore.isAvailable"
                :disabled="isUpdatingAvailability || isCheckingTransfers"
                @update:checked="handleAvailabilityChange"
              />
            </div>
            <Separator class="my-1" />
            <RouterLink to="/profile">
              <Button
                variant="ghost"
                class="w-full justify-start px-2 py-1 h-auto text-[13px] font-normal"
                @click="isUserMenuOpen = false"
              >
                <User class="mr-2 h-3.5 w-3.5" />
                <span>Profile</span>
              </Button>
            </RouterLink>
            <Separator class="my-1" />
            <div class="text-xs font-medium px-2 py-1 text-muted-foreground">Theme</div>
            <div class="flex gap-0.5 px-1.5 py-1">
              <Button
                variant="ghost"
                size="icon"
                class="h-7 w-7"
                :class="colorMode === 'light' && 'bg-accent'"
                @click="setColorMode('light')"
              >
                <Sun class="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="h-7 w-7"
                :class="colorMode === 'dark' && 'bg-accent'"
                @click="setColorMode('dark')"
              >
                <Moon class="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="h-7 w-7"
                :class="colorMode === 'system' && 'bg-accent'"
                @click="setColorMode('system')"
              >
                <Monitor class="h-3.5 w-3.5" />
              </Button>
            </div>
            <Separator class="my-1" />
            <Button
              variant="ghost"
              class="w-full justify-start px-2 py-1 h-auto text-[13px] font-normal"
              @click="handleLogout"
            >
              <LogOut class="mr-2 h-3.5 w-3.5" />
              <span>Log out</span>
            </Button>
          </PopoverContent>
        </Popover>
      </div>
    </aside>

    <!-- Main content -->
    <main class="flex-1 overflow-hidden">
      <RouterView />
    </main>

    <!-- Away Warning Dialog -->
    <AlertDialog :open="showAwayWarning">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Active Transfers Will Be Returned to Queue</AlertDialogTitle>
          <AlertDialogDescription>
            You have {{ awayWarningTransferCount }} active transfer(s) assigned to you.
            Setting your status to "Away" will return them to the queue for other agents to pick up.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button variant="outline" @click="showAwayWarning = false">Cancel</Button>
          <Button @click="confirmGoAway" :disabled="isUpdatingAvailability">Go Away</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
