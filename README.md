# KPM · Kotlin Package Manager

CLI de Go para agregar dependencias a proyectos Kotlin Toolchain.

```powershell
go install .
kpm version
```

Go instala `kpm.exe` en `GOBIN` o, si no está configurado, en `GOPATH/bin`. Agrega esa carpeta a tu PATH.
Desde la raíz de un proyecto con `project.yaml`, o desde una subcarpeta:

```powershell
kpm add coil-compose
kpm add coil-compose coil-network-okhttp -m shared
kpm add io.coil-kt.coil3:coil-compose
kpm add io.coil-kt.coil3:coil-compose:3.6.3
```

El módulo predeterminado es `shared`. Un nombre corto busca el artefacto exacto
en Maven Central y pide elegir el grupo si hay varias coincidencias. Si omites
la versión, se consulta el metadato del artefacto y se guarda una versión estable
explícita. Para versiones con una convención distinta o repositorios distintos
de Maven Central, proporciona la coordenada completa.

La CLI evita duplicados, rechaza conflictos de versión explícitos y ejecuta
`kotlin build` una vez tras procesar todos los artefactos. Si el build falla,
conserva el cambio en `module.yaml` y devuelve el código de salida del wrapper.

## Repositorio y autoría

- Repositorio oficial en [GitHub](https://github.com/fgp555/kotlin-package-manager).
- Creado por [Frank GP](https://frankgp.com).
