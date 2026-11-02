import java.util.Properties

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
}

android {
    namespace = "io.github.geckoterm.android"
    compileSdk = 35

    defaultConfig {
        // Build id: commit (-dirty with uncommitted changes) and build time
        // (UTC), from app/build-id.properties that `make android-gecko`
        // writes; a plain Gradle build says "unknown".
        val buildId = Properties().apply {
            file("build-id.properties").takeIf { it.exists() }?.reader()?.use { load(it) }
        }
        buildConfigField("String", "BUILD_COMMIT", "\"${buildId.getProperty("commit", "unknown")}\"")
        buildConfigField("String", "BUILD_TIME", "\"${buildId.getProperty("time", "")}\"")
        applicationId = "io.github.geckoterm.android"
        minSdk = 26
        targetSdk = 35
        versionCode = 8
        versionName = (project.findProperty("geckoVersion") as String?) ?: "0.3.3"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        // Instrumented tests connect to a Gecko daemon when given one:
        // -Pandroid.testInstrumentationRunnerArguments.geckoUrl=http://10.0.2.2:7810/?token=...
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    buildFeatures {
        buildConfig = true
    }
    packaging {
        // Gecko's daemon ships as lib/<abi>/libgecko.so: native libraries are
        // the only files Android lets an app execute, and only when they are
        // extracted on install.
        jniLibs {
            useLegacyPackaging = true
        }
    }
    testOptions {
        unitTests.isReturnDefaultValues = true
    }
}

kotlin {
    jvmToolchain(17)
}

dependencies {
    implementation(libs.androidx.core)
    implementation(libs.androidx.activity)
    implementation(libs.androidx.appcompat)
    implementation(libs.androidx.webkit)
    implementation(libs.androidx.recyclerview)
    implementation(libs.material)
    implementation(libs.okhttp)
    implementation(libs.zxing.embedded)
    implementation(libs.zxing.core)

    testImplementation(libs.junit)
    testImplementation(libs.json)

    androidTestImplementation(libs.androidx.test.junit)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.rules)
    androidTestImplementation(libs.espresso.core)
    androidTestImplementation(libs.espresso.web)
    androidTestImplementation(libs.uiautomator)
}

// libgecko.so is Gecko itself, cross-compiled by `make android-gecko`.
val geckoLibs = listOf("arm64-v8a", "x86_64").map { file("src/main/jniLibs/$it/libgecko.so") }
tasks.named("preBuild") {
    doFirst {
        val missing = geckoLibs.filter { !it.exists() }
        if (missing.isNotEmpty()) {
            throw GradleException("Gecko for Android is missing (${missing.joinToString()}). Run `make android` (or `make android-gecko`) in the repository root.")
        }
    }
}
