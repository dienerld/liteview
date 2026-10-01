<script setup lang="ts">
import { onMounted } from "vue"
import { Toaster } from "@/components/ui/sonner"
import AppSidebar from "@/components/AppSidebar.vue"
import EmptyState from "@/components/EmptyState.vue"
import DataTab from "@/components/DataTab.vue"
import StructureTab from "@/components/StructureTab.vue"
import { Breadcrumb, BreadcrumbItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from "@/components/ui/breadcrumb"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useViewer } from "@/stores/viewer"

const store = useViewer()
onMounted(() => store.init())
</script>

<template>
  <div class="bg-background text-foreground flex h-screen">
    <EmptyState v-if="!store.db" class="flex-1" />
    <template v-else>
      <AppSidebar />
      <main class="flex min-w-0 flex-1 flex-col overflow-hidden">
        <p v-if="!store.current" class="text-muted-foreground p-8">Escolha uma tabela na barra lateral.</p>
        <template v-else>
          <Breadcrumb class="border-b px-4 py-2">
            <BreadcrumbList>
              <template v-for="(e, i) in store.trail" :key="i">
                <BreadcrumbSeparator v-if="i > 0" />
                <BreadcrumbItem>
                  <BreadcrumbPage v-if="i === store.trail.length - 1">{{ e.label }}</BreadcrumbPage>
                  <BreadcrumbLink v-else as="button" @click="store.backTo(i)">{{ e.label }}</BreadcrumbLink>
                </BreadcrumbItem>
              </template>
            </BreadcrumbList>
          </Breadcrumb>
          <Tabs default-value="data" class="flex min-h-0 flex-1 flex-col">
            <TabsList class="mx-4 mt-2 w-fit">
              <TabsTrigger value="data">Dados</TabsTrigger>
              <TabsTrigger value="structure">Estrutura</TabsTrigger>
            </TabsList>
            <TabsContent value="data" force-mount class="min-h-0 flex-1 data-[state=inactive]:hidden">
              <DataTab :key="store.trail.length + store.current.label" :entry="store.current" />
            </TabsContent>
            <TabsContent value="structure" class="min-h-0 flex-1 overflow-auto">
              <StructureTab :table="store.current.table" />
            </TabsContent>
          </Tabs>
        </template>
      </main>
    </template>
    <Toaster />
  </div>
</template>
