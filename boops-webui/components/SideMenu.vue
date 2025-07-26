<template>
    <v-list>
        <v-list-item rounded="sm" link="link" nuxt="nuxt" to="/">
            <template v-slot:prepend="">
                <v-icon icon="mdi-home"></v-icon>
            </template>
            <v-list-item-title>トップ</v-list-item-title>
        </v-list-item>
        <v-list-subheader>Service設定(フロント設定)</v-list-subheader>
        <v-list-item rounded="sm" link="link" nuxt="nuxt" to="/list-service"><template v-slot:prepend=""><v-icon icon="mdi-server"></v-icon></template>
            <v-list-item-title>設定一覧</v-list-item-title>
        </v-list-item>
        <v-list-item rounded="sm" link="link" nuxt="nuxt" to="/new-service"><template v-slot:prepend=""><v-icon icon="mdi-server-plus"></v-icon></template>
            <v-list-item-title>新規作成</v-list-item-title>
        </v-list-item>
        <v-list-subheader>Upstream設定(バックエンド経路)</v-list-subheader>
        <v-list-item rounded="sm" link="link" nuxt="nuxt" to="/list-upstream"><template v-slot:prepend=""><v-icon icon="mdi-source-fork"></v-icon></template>
            <v-list-item-title>設定一覧</v-list-item-title>
        </v-list-item>
        <v-list-item rounded="sm" link="link" nuxt="nuxt" to="/new-upstream"><template v-slot:prepend=""><v-icon icon="mdi-source-branch-plus"></v-icon></template>
            <v-list-item-title>新規作成</v-list-item-title>
        </v-list-item>
        <v-list-subheader>証明書設定</v-list-subheader>
        <v-list-item rounded="sm" link="link" nuxt="nuxt" to="/list-certificate"><template v-slot:prepend=""><v-icon icon="mdi-certificate"></v-icon></template>
            <v-list-item-title>SSL証明書</v-list-item-title>
        </v-list-item>
    </v-list>
</template>