// MyGo adds this Android library and sets compileSdk/minSdk/Java/Kotlin and
// the dependency on its host library. Keep manifest and resources in this
// library so Android's manifest/resource merger carries them into the APK.
plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "mygo_player.android.native_.localplayback"
}
