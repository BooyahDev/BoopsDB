<template>
    <div style="display: inline-block; vertical-align: middle;">
        <ul>
            <div style="display: inline-block;vertical-align: middle;">
                <a href="https://www.booyah.dev">
                    <!-- <img class="mr-2" src="/logo_white.png" style="display: inline-block;vertical-align: middle; height: 40px;" /> -->
                </a>
            </div>
            <slot name="title">BoopsDB
            </slot>
            <a href="/" style="color: white; text-decoration: none;" class="ml-1 mr-1">検索</a> | 
            <a href="/machines/register" style="color: white; text-decoration: none">マシン登録</a>
        </ul>
    </div>
</template>

<script setup>
</script>