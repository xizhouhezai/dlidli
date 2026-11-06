pluginManagement {
    val flutterSdkPath = run {
        val properties = java.util.Properties()
        file("local.properties").inputStream().use { properties.load(it) }
        val flutterSdkPath = properties.getProperty("flutter.sdk")
        require(flutterSdkPath != null) { "flutter.sdk not set in local.properties" }
        flutterSdkPath
    }

    includeBuild("$flutterSdkPath/packages/flutter_tools/gradle")

    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

plugins {
    id("dev.flutter.flutter-plugin-loader") version "1.0.0"
    id("com.android.application") version "8.7.0" apply false
    // Kotlin 1.8.22：定案用 `video_player`（ExoPlayer）后**无需升级**。
    // 曾因评测 `media_kit` 临时升到 2.1.0（其依赖用 Kotlin 2.2.0 编译，
    // 1.8 会报 "incompatible version of Kotlin"）；定案后已回退。
    id("org.jetbrains.kotlin.android") version "1.8.22" apply false
}

include(":app")
