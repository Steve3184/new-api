/* Copyright (C) 2023-2026 QuantumNous */
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as THREE from 'three'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js'

type ThreeDViewerProps = {
  url: string
}

export function ThreeDViewer(props: ThreeDViewerProps) {
  const { t } = useTranslation()
  const containerRef = useRef<HTMLDivElement>(null)
  const [loadFailed, setLoadFailed] = useState(false)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const scene = new THREE.Scene()
    const camera = new THREE.PerspectiveCamera(42, 1, 0.01, 1000)
    camera.position.set(2.8, 2.1, 3.6)
    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true })
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
    renderer.outputColorSpace = THREE.SRGBColorSpace
    renderer.toneMapping = THREE.ACESFilmicToneMapping
    renderer.toneMappingExposure = 1.05
    container.appendChild(renderer.domElement)

    const controls = new OrbitControls(camera, renderer.domElement)
    controls.enableDamping = true
    controls.minDistance = 0.2
    controls.maxDistance = 30
    scene.add(new THREE.HemisphereLight(0xffffff, 0x30343b, 2.3))
    const keyLight = new THREE.DirectionalLight(0xffffff, 3.2)
    keyLight.position.set(4, 6, 5)
    scene.add(keyLight)
    const fillLight = new THREE.DirectionalLight(0x9cc8ff, 1.2)
    fillLight.position.set(-4, 2, -3)
    scene.add(fillLight)
    const grid = new THREE.GridHelper(20, 20, 0x71717a, 0x3f3f46)
    grid.material.transparent = true
    grid.material.opacity = 0.25
    scene.add(grid)

    let modelRoot: THREE.Object3D | null = null
    let frame = 0
    const resize = () => {
      const width = Math.max(container.clientWidth, 1)
      const height = Math.max(container.clientHeight, 1)
      renderer.setSize(width, height, false)
      camera.aspect = width / height
      camera.updateProjectionMatrix()
    }
    const observer = new ResizeObserver(resize)
    observer.observe(container)
    resize()

    const loader = new GLTFLoader()
    loader.load(
      props.url,
      (gltf) => {
        modelRoot = gltf.scene
        scene.add(gltf.scene)
        const box = new THREE.Box3().setFromObject(gltf.scene)
        const size = box.getSize(new THREE.Vector3())
        const center = box.getCenter(new THREE.Vector3())
        gltf.scene.position.sub(center)
        gltf.scene.position.y += size.y / 2
        const radius = Math.max(size.x, size.y, size.z, 0.1)
        camera.position.set(radius * 1.8, radius * 1.25, radius * 2.2)
        controls.target.set(0, size.y / 2, 0)
        controls.update()
      },
      undefined,
      () => setLoadFailed(true)
    )

    const render = () => {
      controls.update()
      renderer.render(scene, camera)
      frame = window.requestAnimationFrame(render)
    }
    render()

    return () => {
      window.cancelAnimationFrame(frame)
      observer.disconnect()
      controls.dispose()
      if (modelRoot) {
        modelRoot.traverse((object) => {
          if (!(object instanceof THREE.Mesh)) return
          object.geometry.dispose()
          const materials = Array.isArray(object.material)
            ? object.material
            : [object.material]
          for (const material of materials) material.dispose()
        })
      }
      renderer.dispose()
      renderer.domElement.remove()
    }
  }, [props.url])

  return (
    <div className='bg-muted/30 relative h-[clamp(28rem,68vh,52rem)] w-full overflow-hidden'>
      <div ref={containerRef} className='size-full' />
      {loadFailed && (
        <div className='bg-background/80 absolute inset-0 grid place-items-center p-6 text-center text-sm backdrop-blur-sm'>
          {t('Failed to load 3D model.')}
        </div>
      )}
    </div>
  )
}
