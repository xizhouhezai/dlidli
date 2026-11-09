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
    // Kotlin 2.1.0：`screen_brightness`（T-12 手势调亮度）用到了
    // KotlinAndroidProjectExtension.compilerOptions(Function1)，该 API 是 KGP 2.x 才有的。
    // 停留在 1.8.22 会在构建期直接报 "What went wrong: 'void ...compilerOptions(...)'"。
    // 历史：曾因评测 media_kit 升到 2.1.0 后回退；这次是 T-12 的硬性依赖，保留 2.1.0。
    id("org.jetbrains.kotlin.android") version "2.1.0" apply false
}

include(":app")
